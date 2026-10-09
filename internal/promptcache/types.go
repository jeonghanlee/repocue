// Package promptcache defines provider-independent retained prompt-cache evidence.
package promptcache

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jeonghanlee/repocue/docs/schema"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

const (
	BenchmarkVersion         = "repocue/cache-benchmark-1"
	RequestVersion           = "repocue/cache-request-1"
	ObservationVersion       = "repocue/cache-observation-1"
	AnswerVersion            = "repocue/cache-answer-1"
	EvaluationVersion        = "repocue/cache-evaluation-1"
	MaxCounter         int64 = 9007199254740991
)

type Provider struct {
	Model      string `json:"model"`
	APIVersion string `json:"api_version"`
	TTL        string `json:"ttl"`
}
type Limits struct {
	MaxRequests      int `json:"max_requests"`
	MaxOutputTokens  int `json:"max_output_tokens"`
	TimeoutMS        int `json:"timeout_ms"`
	MaxResponseBytes int `json:"max_response_bytes"`
}
type Rates struct {
	ID         string `json:"id"`
	AsOf       string `json:"as_of"`
	Unit       string `json:"unit"`
	Model      string `json:"model"`
	APIVersion string `json:"api_version"`
	Input      string `json:"input"`
	Write5m    string `json:"write_5m"`
	Write1h    string `json:"write_1h"`
	Read       string `json:"read"`
	Output     string `json:"output"`
}
type Step struct {
	ID        string `json:"id"`
	PairID    string `json:"pair_id"`
	Arm       string `json:"arm"`
	Condition string `json:"condition"`
	Prompt    string `json:"prompt"`
}
type Spec struct {
	SchemaVersion    string   `json:"schema_version"`
	ID               string   `json:"id"`
	TaskKind         string   `json:"task_kind"`
	ContextFile      string   `json:"context_file"`
	Targets          []string `json:"targets"`
	Rubric           string   `json:"rubric"`
	MaxContextTokens int      `json:"max_context_tokens"`
	Provider         Provider `json:"provider"`
	Limits           Limits   `json:"limits"`
	Steps            []Step   `json:"steps"`
	Rates            *Rates   `json:"rates"`
}
type Request struct {
	SchemaVersion      string `json:"schema_version"`
	StepID             string `json:"step_id"`
	BenchmarkDigest    string `json:"benchmark_digest"`
	CueDigest          string `json:"cue_digest"`
	AnswerSchemaDigest string `json:"answer_schema_digest"`
	BasisFingerprint   string `json:"basis_fingerprint"`
	PrefixDigest       string `json:"prefix_digest"`
	WireDigest         string `json:"wire_digest"`
	WireBody           string `json:"wire_body"`
}
type Claim struct {
	Text      string   `json:"text"`
	SourceIDs []string `json:"source_ids"`
}
type Answer struct {
	SchemaVersion string   `json:"schema_version"`
	TaskID        string   `json:"task_id"`
	RequiredIDs   []string `json:"required_ids"`
	SourceIDs     []string `json:"source_ids"`
	Claims        []Claim  `json:"claims"`
	Uncertainties []string `json:"uncertainties"`
}
type Assessment struct {
	StepID           string   `json:"step_id"`
	BenchmarkDigest  string   `json:"benchmark_digest"`
	RequestDigest    string   `json:"request_digest"`
	ResponseDigest   string   `json:"response_digest"`
	AnswerDigest     string   `json:"answer_digest"`
	BasisFingerprint string   `json:"basis_fingerprint"`
	SourceIDs        []string `json:"source_ids"`
	Assessor         string   `json:"assessor"`
	Rubric           string   `json:"rubric"`
	Correct          bool     `json:"correct"`
	Compliant        *bool    `json:"compliant"`
}

// SupportedModels is deliberately bounded to documented structured-output
// models. Unknown IDs fail before execution; this is not an API availability check.
var supportedModels = map[string]bool{
	"claude-sonnet-4-6": true, "claude-opus-4-6": true, "claude-opus-4-7": true,
	"claude-sonnet-4-5-20250929": true, "claude-opus-4-5-20251101": true,
	"claude-haiku-4-5-20251001": true,
}

func DecodeSpec(data []byte) (Spec, error) {
	var result Spec
	if err := taskcontext.DecodeStrict(data, "cache-benchmark-v1.schema.json", &result); err != nil {
		return result, err
	}
	if !supportedModels[result.Provider.Model] {
		return result, errors.New("model has no supported frozen structured-output profile")
	}
	if err := taskcontext.ValidatePath(result.ContextFile, false); err != nil {
		return result, err
	}
	for _, target := range result.Targets {
		if err := taskcontext.ValidatePath(target, true); err != nil {
			return result, err
		}
	}
	if len(result.Steps) > result.Limits.MaxRequests {
		return result, errors.New("steps exceed max_requests")
	}
	seen := map[string]bool{}
	for _, step := range result.Steps {
		if seen[step.ID] {
			return result, errors.New("step IDs must be unique")
		}
		seen[step.ID] = true
		if (step.Arm == "baseline") != (step.Condition == "baseline") {
			return result, errors.New("baseline arm must disable caching")
		}
		if step.Arm == "cache" && step.Condition != "create" && step.Condition != "reuse" {
			return result, errors.New("cache cycles contain only creation/reuse intentions")
		}
		if step.Arm == "probe" && step.Condition != "changed-prefix" && step.Condition != "expiry" {
			return result, errors.New("probes must declare changed-prefix or expiry")
		}
	}
	if result.Rates != nil {
		if err := ValidateRates(*result.Rates, result.Provider, time.Now().UTC()); err != nil {
			return result, err
		}
	}
	return result, nil
}

func ValidateRates(rates Rates, provider Provider, at time.Time) error {
	data, err := json.Marshal(rates)
	if err != nil {
		return err
	}
	if err := taskcontext.DecodeStrict(data, "cache-benchmark-v1.schema.json#/properties/rates/anyOf/0", &rates); err != nil {
		return err
	}
	date, err := time.Parse("2006-01-02", rates.AsOf)
	if err != nil || date.After(at.UTC()) || rates.Model != provider.Model || rates.APIVersion != provider.APIVersion {
		return errors.New("rates require a nonfuture date and matching model/API version")
	}
	return nil
}

func AnswerSchema() ([]byte, error) {
	raw, err := schema.Contract("cache-answer-v1.schema.json")
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	delete(value, "$schema")
	delete(value, "$id")
	delete(value, "title")
	return json.Marshal(value)
}
func SameIDs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a, b := append([]string{}, left...), append([]string{}, right...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] || i > 0 && a[i] == a[i-1] {
			return false
		}
	}
	return true
}
func RequiredSources(ids []string, known map[string]bool) error {
	seen := map[string]bool{}
	for _, id := range ids {
		if !known[id] || seen[id] {
			return fmt.Errorf("unknown or duplicate source ID: %s", id)
		}
		seen[id] = true
	}
	return nil
}
