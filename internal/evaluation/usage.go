package evaluation

import (
	"errors"
	"fmt"
	"math"
)

// TokenMetricsFromUsage totals each counter only when every completed turn
// reports it. Cached and reasoning tokens are subsets, not additional totals.
func TokenMetricsFromUsage(events []UsageEvent) (M2RunnerMetrics, error) {
	if len(events) == 0 {
		return M2RunnerMetrics{}, errors.New("runner observation requires at least one completed turn")
	}
	for index, event := range events {
		if event.Turn != index+1 {
			return M2RunnerMetrics{}, errors.New("runner usage events must be ordered completed turns")
		}
		if event.CachedInputTokens != nil && event.InputTokens != nil && *event.CachedInputTokens > *event.InputTokens {
			return M2RunnerMetrics{}, errors.New("cached input tokens cannot exceed input tokens")
		}
	}
	metrics := M2RunnerMetrics{Statuses: map[string]string{}}
	for _, counter := range []struct {
		name   string
		read   func(UsageEvent) *int64
		target **int64
	}{
		{"input_tokens", func(e UsageEvent) *int64 { return e.InputTokens }, &metrics.InputTokens},
		{"cached_input_tokens", func(e UsageEvent) *int64 { return e.CachedInputTokens }, &metrics.CachedInputTokens},
		{"output_tokens", func(e UsageEvent) *int64 { return e.OutputTokens }, &metrics.OutputTokens},
		{"reasoning_output_tokens", func(e UsageEvent) *int64 { return e.ReasoningOutputTokens }, &metrics.ReasoningOutputTokens},
	} {
		var total int64
		complete := true
		for _, event := range events {
			value := counter.read(event)
			if value == nil {
				complete = false
				continue
			}
			if *value < 0 || *value > math.MaxInt64-total {
				return M2RunnerMetrics{}, fmt.Errorf("runner usage %s is negative or overflows int64", counter.name)
			}
			total += *value
		}
		metrics.Statuses[counter.name] = measurementUnobserved
		if complete {
			*counter.target = &total
			metrics.Statuses[counter.name] = measurementObserved
		}
	}
	metrics.Statuses["total_tokens"] = measurementUnobserved
	if metrics.InputTokens != nil && metrics.OutputTokens != nil {
		if *metrics.OutputTokens > math.MaxInt64-*metrics.InputTokens {
			return M2RunnerMetrics{}, errors.New("runner total tokens overflow int64")
		}
		total := *metrics.InputTokens + *metrics.OutputTokens
		metrics.TotalTokens = &total
		metrics.Statuses["total_tokens"] = measurementDerived
	}
	return metrics, nil
}

func tokenValues(metrics M2RunnerMetrics) map[string]*int64 {
	return map[string]*int64{
		"input_tokens": metrics.InputTokens, "cached_input_tokens": metrics.CachedInputTokens,
		"output_tokens": metrics.OutputTokens, "reasoning_output_tokens": metrics.ReasoningOutputTokens,
		"total_tokens": metrics.TotalTokens,
	}
}
