package evaluation

import (
	"encoding/json"
	"math"
	"time"

	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
)

type TaskMetric struct {
	Status string   `json:"status"`
	Value  *float64 `json:"value"`
	Unit   string   `json:"unit"`
	Reason string   `json:"reason"`
}

func taskUnknown(unit, reason string) TaskMetric {
	return TaskMetric{Status: "not_observed", Unit: unit, Reason: reason}
}
func taskMeasured(value float64, status, unit, reason string) TaskMetric {
	return TaskMetric{Status: status, Value: &value, Unit: unit, Reason: reason}
}

func TaskCredit(transport taskbenchmark.Transport, rates *taskbenchmark.Rates, settings taskbenchmark.Settings, at time.Time) TaskMetric {
	missing := taskUnknown("credits", "complete compatible usage and dated rates are required")
	if rates == nil || !transport.UsageComplete || transport.Status != "completed" ||
		taskbenchmark.ValidateRates(*rates, at) != nil || rates.Model != settings.Model || rates.SpeedTier != settings.SpeedTier {
		return missing
	}
	usage, err := taskTokenMetrics(transport.UsageEvents)
	if err != nil || usage.InputTokens == nil || usage.CachedInputTokens == nil || usage.OutputTokens == nil {
		return missing
	}
	value := (float64(*usage.InputTokens-*usage.CachedInputTokens)*rates.Input +
		float64(*usage.CachedInputTokens)*rates.CachedInput + float64(*usage.OutputTokens)*rates.Output) / 1000000
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return missing
	}
	return taskMeasured(value, "estimated", "credits", "complete usage with frozen per-million-token credit rates")
}

func taskTokenMetrics(events []taskbenchmark.Usage) (M2RunnerMetrics, error) {
	legacy := make([]UsageEvent, 0, len(events))
	for _, event := range events {
		if event.ReasoningOutputTokens != nil && event.OutputTokens != nil && *event.ReasoningOutputTokens > *event.OutputTokens {
			return M2RunnerMetrics{}, errTaskReasoningBounds
		}
		legacy = append(legacy, UsageEvent{Turn: event.Turn, InputTokens: event.InputTokens,
			CachedInputTokens: event.CachedInputTokens, OutputTokens: event.OutputTokens, ReasoningOutputTokens: event.ReasoningOutputTokens})
	}
	metrics, err := TokenMetricsFromUsage(legacy)
	if err == nil && metrics.ReasoningOutputTokens != nil && metrics.OutputTokens != nil && *metrics.ReasoningOutputTokens > *metrics.OutputTokens {
		return M2RunnerMetrics{}, errTaskReasoningBounds
	}
	return metrics, err
}

func taskMetrics(transport *taskbenchmark.Transport, rates *taskbenchmark.Rates, settings taskbenchmark.Settings, at time.Time) map[string]TaskMetric {
	result := map[string]TaskMetric{}
	for name, unit := range map[string]string{"input_tokens": "tokens", "cached_input_tokens": "tokens", "output_tokens": "tokens",
		"reasoning_output_tokens": "tokens", "total_tokens": "tokens", "credit_estimate": "credits",
		"agent_duration_ms": "ms", "command_calls": "calls", "cpu_ms": "ms", "memory_bytes": "bytes",
		"filesystem_io_bytes": "bytes", "task_ready_ms": "ms", "account_billing_credits": "credits",
		"cue_duration_ms": "ms", "cue_bytes": "bytes", "cue_estimated_tokens": "tokens"} {
		result[name] = taskUnknown(unit, "no complete measurement path")
	}
	if transport == nil {
		return result
	}
	result["agent_duration_ms"] = taskMeasured(transport.DurationMS, "observed", "ms", "full adapter wall time; not task-ready latency")
	result["command_calls"] = taskMeasured(float64(len(transport.Commands)), "derived", "calls", "supported completed command events; not all tool calls")
	result["credit_estimate"] = TaskCredit(*transport, rates, settings, at)
	if !transport.UsageComplete {
		return result
	}
	metrics, err := taskTokenMetrics(transport.UsageEvents)
	if err != nil {
		return result
	}
	for name, value := range map[string]*int64{"input_tokens": metrics.InputTokens, "cached_input_tokens": metrics.CachedInputTokens,
		"output_tokens": metrics.OutputTokens, "reasoning_output_tokens": metrics.ReasoningOutputTokens, "total_tokens": metrics.TotalTokens} {
		if value != nil {
			result[name] = taskMeasured(float64(*value), metrics.Statuses[name], "tokens", "complete-turn counter coverage")
		}
	}
	return result
}

func taskRateDigest(rates *taskbenchmark.Rates) string {
	if rates == nil {
		return ""
	}
	serialized, _ := json.Marshal(rates)
	return taskDigest(serialized)
}
