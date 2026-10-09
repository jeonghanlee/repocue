package promptcache

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

type Usage struct {
	OrdinaryInput *int64 `json:"ordinary_input"`
	CacheWrite    *int64 `json:"cache_write"`
	CacheWrite5m  *int64 `json:"cache_write_5m"`
	CacheWrite1h  *int64 `json:"cache_write_1h"`
	CacheRead     *int64 `json:"cache_read"`
	Output        *int64 `json:"output"`
	TotalInput    *int64 `json:"total_input"`
}
type Response struct {
	Model       string
	StopReason  string
	MessageID   string
	Usage       Usage
	Answer      *Answer
	AnswerError string
}

func counter(values map[string]json.RawMessage, name string) (*int64, error) {
	raw, ok := values[name]
	if !ok || string(raw) == "null" {
		return nil, nil
	}
	var value int64
	if err := json.Unmarshal(raw, &value); err != nil || value < 0 || value > MaxCounter {
		return nil, fmt.Errorf("invalid provider counter: %s", name)
	}
	return &value, nil
}
func sum(values ...*int64) (*int64, error) {
	var total int64
	for _, value := range values {
		if value == nil {
			return nil, nil
		}
		if *value < 0 || *value > MaxCounter-total {
			return nil, errors.New("provider counter total overflows supported bounds")
		}
		total += *value
	}
	return &total, nil
}

// DecodeResponse preserves absent counters and checks aggregate consistency.
// Provider extensions are tolerated, while duplicate members remain invalid.
func DecodeResponse(raw []byte, ttl string) (Response, error) {
	var root map[string]json.RawMessage
	result := Response{}
	if err := taskcontext.DecodeStrict(raw, "", &root); err != nil {
		return result, err
	}
	_ = json.Unmarshal(root["model"], &result.Model)
	_ = json.Unmarshal(root["stop_reason"], &result.StopReason)
	_ = json.Unmarshal(root["id"], &result.MessageID)
	var values map[string]json.RawMessage
	if data := root["usage"]; len(data) > 0 && string(data) != "null" {
		if err := json.Unmarshal(data, &values); err != nil {
			return result, errors.New("provider usage is not an object")
		}
	}
	var err error
	for _, field := range []struct {
		name   string
		target **int64
	}{
		{"input_tokens", &result.Usage.OrdinaryInput}, {"cache_creation_input_tokens", &result.Usage.CacheWrite},
		{"cache_read_input_tokens", &result.Usage.CacheRead}, {"output_tokens", &result.Usage.Output},
	} {
		*field.target, err = counter(values, field.name)
		if err != nil {
			return result, err
		}
	}
	var lifetime map[string]json.RawMessage
	if data := values["cache_creation"]; len(data) > 0 && string(data) != "null" {
		if err := json.Unmarshal(data, &lifetime); err != nil {
			return result, errors.New("cache_creation is not an object")
		}
	}
	result.Usage.CacheWrite5m, err = counter(lifetime, "ephemeral_5m_input_tokens")
	if err != nil {
		return result, err
	}
	result.Usage.CacheWrite1h, err = counter(lifetime, "ephemeral_1h_input_tokens")
	if err != nil {
		return result, err
	}
	detail, err := sum(result.Usage.CacheWrite5m, result.Usage.CacheWrite1h)
	if err != nil {
		return result, err
	}
	aggregate := result.Usage.CacheWrite
	if detail != nil && aggregate != nil && *detail != *aggregate {
		return result, errors.New("cache lifetime sum differs from aggregate")
	}
	for _, part := range []*int64{result.Usage.CacheWrite5m, result.Usage.CacheWrite1h} {
		if part != nil && aggregate != nil && *part > *aggregate {
			return result, errors.New("cache lifetime counter exceeds aggregate")
		}
	}
	// A single explicit TTL can allocate an otherwise undetailed aggregate.
	// A partial lifetime breakdown is kept unknown rather than overwritten.
	if aggregate != nil && result.Usage.CacheWrite5m == nil && result.Usage.CacheWrite1h == nil {
		zero := int64(0)
		if *aggregate == 0 {
			result.Usage.CacheWrite5m = &zero
			result.Usage.CacheWrite1h = &zero
		} else if ttl == "5m" {
			result.Usage.CacheWrite5m = aggregate
			result.Usage.CacheWrite1h = &zero
		} else if ttl == "1h" {
			result.Usage.CacheWrite1h = aggregate
			result.Usage.CacheWrite5m = &zero
		}
	}
	creation := aggregate
	if creation == nil {
		creation = detail
	}
	result.Usage.TotalInput, err = sum(result.Usage.OrdinaryInput, creation, result.Usage.CacheRead)
	if err != nil {
		return result, err
	}
	if details := values["output_tokens_details"]; len(details) > 0 && string(details) != "null" {
		var d map[string]json.RawMessage
		if err := json.Unmarshal(details, &d); err != nil {
			return result, err
		}
		reasoning, err := counter(d, "thinking_tokens")
		if err != nil {
			return result, err
		}
		if reasoning != nil && result.Usage.Output != nil && *reasoning > *result.Usage.Output {
			return result, errors.New("thinking tokens exceed output")
		}
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(root["content"], &blocks); err != nil || len(blocks) != 1 || blocks[0].Type != "text" {
		result.AnswerError = "response does not contain one structured text answer"
	} else if result.StopReason != "end_turn" {
		result.AnswerError = "response did not finish with end_turn"
	} else {
		var answer Answer
		if err := taskcontext.DecodeStrict([]byte(blocks[0].Text), "cache-answer-v1.schema.json", &answer); err != nil {
			result.AnswerError = "answer violates the frozen local contract"
		} else {
			result.Answer = &answer
		}
	}
	return result, nil
}
func (usage Usage) Activity() string {
	writes := usage.CacheWrite
	if writes == nil {
		writes, _ = sum(usage.CacheWrite5m, usage.CacheWrite1h)
	}
	if usage.CacheRead != nil && *usage.CacheRead > 0 {
		if writes != nil && *writes > 0 {
			return "creation-and-reuse"
		}
		return "reuse"
	}
	if writes != nil && *writes > 0 {
		return "creation"
	}
	if writes != nil && *writes == 0 && usage.CacheRead != nil && *usage.CacheRead == 0 {
		return "none-observed"
	}
	return "not_observed"
}
