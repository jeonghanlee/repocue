package reporting

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func number(value float64) *float64 { return &value }

func measured(status string, value float64, unit, basis string) Measurement {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return missing(unit, basis, "arithmetic-overflow")
	}
	return Measurement{Status: status, Value: number(value), Unit: unit, Basis: basis, Limitations: []string{}}
}

func missing(unit, basis, reason string) Measurement {
	return Measurement{Status: "not_observed", Unit: unit, Basis: basis, Limitations: []string{reason}}
}

func summarize(values []float64, unknown int, status, unit, basis string) Statistic {
	result := Statistic{Status: status, Unit: unit, Basis: basis, Values: append([]float64{}, values...), Count: len(values), Unknown: unknown, Coverage: "complete"}
	if unknown > 0 {
		result.Coverage = "partial"
	}
	if len(values) == 0 {
		result.Status = "not_observed"
		return result
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = sorted[len(sorted)/2-1]/2 + median/2
	}
	total := 0.0
	for _, value := range sorted {
		total += value
	}
	result.Median, result.Minimum, result.Maximum = number(median), number(sorted[0]), number(sorted[len(sorted)-1])
	if unknown == 0 && !math.IsInf(total, 0) {
		result.Total = number(total)
	}
	return result
}

func comparePairs(pairs []Pair, metric string) Comparison {
	result := Comparison{Pairs: []PairDifference{}}
	var direct, basic, differences []float64
	unit, basis := metricBoundary(metric)
	for _, pair := range pairs {
		d, a := sampleMeasurement(pair.Direct, metric), sampleMeasurement(pair.Basic, metric)
		delta := missing(unit, basis, "incomplete-pair")
		if d.Value != nil && a.Value != nil {
			difference := *d.Value - *a.Value
			delta = measured("derived", difference, unit, basis)
			direct, basic, differences = append(direct, *d.Value), append(basic, *a.Value), append(differences, difference)
			if difference > 0 {
				result.PositivePairs++
			}
		}
		result.Pairs = append(result.Pairs, PairDifference{PairID: pair.ID, Direct: d, Basic: a, Difference: delta})
	}
	unknown := len(pairs) - len(differences)
	result.Direct = summarize(direct, unknown, "derived", unit, basis)
	result.Basic = summarize(basic, unknown, "derived", unit, basis)
	result.Difference = summarize(differences, unknown, "derived", unit, "paired direct minus basic; "+basis)
	return result
}

func sampleMeasurement(sample Sample, metric string) Measurement {
	unit, basis := metricBoundary(metric)
	if value, found := sample.Metrics[metric]; found {
		if value.Status == "observed" || value.Status == "derived" || (value.Status == "not_observed" && value.Value == nil) {
			return value
		}
		return missing(unit, basis, "agent-cost-not-observed-or-derived")
	}
	return missing(unit, basis, "agent-measurement-unavailable")
}

func metricBoundary(metric string) (string, string) {
	switch {
	case strings.Contains(metric, "tokens"):
		return "tokens", "agent token usage; cached and reasoning counts are subsets"
	case metric == "execution_duration_ms":
		return "ms", "agent execution duration, not task-ready latency"
	case metric == "git_invocations":
		return "Git processes", "actual Git process invocations"
	case metric == "git_calls":
		return "Git commands", "classified agent command strings, not Git processes"
	case strings.Contains(metric, "proxy_bytes"):
		return "proxy bytes", "named-file-size proxy, not filesystem I/O"
	case strings.Contains(metric, "files_named"):
		return "named files", "explicitly named repository files, not physical reads"
	case metric == "tool_calls":
		return "tool calls", "agent tools only; excludes RepoCue internals"
	default:
		return "commands", "agent commands only; excludes RepoCue internals"
	}
}

func consumerCounts(explicit int64) ([]int64, error) {
	if explicit < 0 || explicit > 1<<53 {
		return nil, fmt.Errorf("consumer scenario exceeds the supported exact integer range")
	}
	counts := []int64{1, 2, 3, 5, 10}
	for _, value := range counts {
		if value == explicit {
			return counts, nil
		}
	}
	if explicit > 0 {
		counts = append(counts, explicit)
		sort.Slice(counts, func(i, j int) bool { return counts[i] < counts[j] })
	}
	return counts, nil
}

func project(pairs []Pair, metric string, consumers int64, explicit bool, maintenance Measurement) Projection {
	unit, basis := metricBoundary(metric)
	result := Projection{Consumers: consumers, Explicit: explicit, Metric: metric, Maintenance: maintenance, Pairs: []PairProjection{}}
	var direct, assisted, savings, percents, finite []float64
	for _, pair := range pairs {
		d, a := sampleMeasurement(pair.Direct, metric), sampleMeasurement(pair.Basic, metric)
		q := measured("derived", 0, unit, "agent-only boundary; cue input tokens are already included")
		if metric == "execution_duration_ms" || metric == "git_invocations" {
			q = missing(unit, "full cue CLI", "full-cue-cost-unavailable")
			if value, found := pair.Cue[metric]; found {
				q = value
			}
		}
		item := PairProjection{PairID: pair.ID, Direct: missing(unit, basis, "incomplete-cost"), Assisted: missing(unit, basis, "incomplete-cost"),
			Saving: missing(unit, basis, "incomplete-cost"), Percent: missing("percent", basis, "incomplete-cost"), BreakEven: missing("consumers", basis, "incomplete-cost")}
		if d.Value != nil && a.Value != nil && q.Value != nil && maintenance.Value != nil && consumers > 0 {
			directTotal := float64(consumers) * *d.Value
			assistedTotal := *maintenance.Value + float64(consumers)*(*a.Value+*q.Value)
			saving := directTotal - assistedTotal
			item.Direct, item.Assisted = measured("estimated", directTotal, unit, basis), measured("estimated", assistedTotal, unit, basis)
			item.Saving = measured("estimated", saving, unit, "paired serial consumer scenario; "+basis)
			if item.Direct.Value != nil && item.Assisted.Value != nil && item.Saving.Value != nil {
				direct, assisted, savings = append(direct, directTotal), append(assisted, assistedTotal), append(savings, saving)
			}
			if directTotal > 0 {
				item.Percent = measured("estimated", 100*(saving/directTotal), "percent", basis)
				if item.Percent.Value != nil {
					percents = append(percents, *item.Percent.Value)
				}
			} else {
				item.Percent = missing("percent", basis, "zero-direct-denominator")
			}
			gap := *d.Value - (*a.Value + *q.Value)
			if gap > 0 && *maintenance.Value >= 0 {
				item.BreakEven = measured("estimated", math.Floor(*maintenance.Value/gap)+1, "consumers", "first strictly cheaper positive consumer count")
				if item.BreakEven.Value != nil {
					finite = append(finite, *item.BreakEven.Value)
				}
			} else {
				item.BreakEven = missing("consumers", basis, "no_finite_break_even")
			}
		}
		result.Pairs = append(result.Pairs, item)
	}
	unknown := len(pairs) - len(savings)
	result.Direct = summarize(direct, unknown, "estimated", unit, basis)
	result.Assisted = summarize(assisted, unknown, "estimated", unit, basis)
	result.Saving = summarize(savings, unknown, "estimated", unit, "median of paired savings, not difference of medians")
	result.Percent = summarize(percents, len(pairs)-len(percents), "estimated", "percent", basis)
	result.FiniteBreakEven = summarize(finite, len(pairs)-len(finite), "estimated", "consumers", "finite pairs only; inspect every pair for unavailable or non-finite results")
	return result
}
