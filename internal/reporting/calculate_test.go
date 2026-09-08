package reporting

import (
	"math"
	"reflect"
	"testing"
)

func costPair(id string, direct, basic float64) Pair {
	return Pair{ID: id, Direct: Sample{Metrics: map[string]Measurement{"total_tokens": measured("observed", direct, "tokens", "test agent boundary")}},
		Basic: Sample{Metrics: map[string]Measurement{"total_tokens": measured("observed", basic, "tokens", "test agent boundary")}}, Cue: map[string]Measurement{}}
}

func TestPairwiseMedianAndProjectedCosts(t *testing.T) {
	pairs := []Pair{costPair("one", 100, 90), costPair("two", 200, 1), costPair("three", 300, 299)}
	comparison := comparePairs(pairs, "total_tokens")
	if comparison.Difference.Median == nil || *comparison.Difference.Median != 10 || *comparison.Direct.Median-*comparison.Basic.Median != 110 {
		t.Fatalf("paired association was lost: %+v", comparison)
	}
	for _, test := range []struct {
		consumers int64
		savings   []float64
		median    float64
	}{
		{1, []float64{-15, 174, -24}, -15}, {2, []float64{-5, 373, -23}, -5}, {3, []float64{5, 572, -22}, 5}, {5, []float64{25, 970, -20}, 25}, {10, []float64{75, 1965, -15}, 75},
	} {
		result := project(pairs, "total_tokens", test.consumers, false, measured("derived", 25, "tokens", "formula test input"))
		if !reflect.DeepEqual(result.Saving.Values, test.savings) || result.Saving.Median == nil || *result.Saving.Median != test.median {
			t.Fatalf("N=%d: %+v", test.consumers, result)
		}
		if result.Saving.Status != "estimated" || !reflect.DeepEqual(result.FiniteBreakEven.Values, []float64{3, 1, 26}) {
			t.Fatalf("wrong boundary/classification: %+v", result)
		}
	}
	if result := summarize([]float64{10, 1}, 0, "derived", "tokens", "even sample"); result.Median == nil || *result.Median != 5.5 {
		t.Fatalf("wrong even median: %+v", result)
	}
}

func TestProjectionUnavailableAndStrictBreakEven(t *testing.T) {
	for _, test := range []struct {
		name             string
		direct, basic, h float64
		want             *float64
		reason           string
	}{
		{"strict equality", 20, 10, 20, number(3), ""},
		{"zero maintenance", 20, 10, 0, number(1), ""},
		{"zero gap", 20, 20, 0, nil, "no_finite_break_even"},
		{"unfavorable", 10, 20, 0, nil, "no_finite_break_even"},
		{"zero direct", 0, 10, 0, nil, "no_finite_break_even"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := project([]Pair{costPair("pair", test.direct, test.basic)}, "total_tokens", 2, false, measured("derived", test.h, "tokens", "test input"))
			value := result.Pairs[0].BreakEven
			if !reflect.DeepEqual(value.Value, test.want) {
				t.Fatalf("wrong break-even: %+v", value)
			}
			if test.reason != "" && (len(value.Limitations) != 1 || value.Limitations[0] != test.reason) {
				t.Fatalf("missing reason: %+v", value)
			}
			if test.direct == 0 && result.Pairs[0].Percent.Value != nil {
				t.Fatal("zero denominator became a percentage")
			}
		})
	}
	pair := costPair("pair", 100, 50)
	delete(pair.Basic.Metrics, "total_tokens")
	result := project([]Pair{pair}, "total_tokens", 1, false, measured("derived", 0, "tokens", "test input"))
	if result.Saving.Median != nil || result.Saving.Unknown != 1 || result.Saving.Status != "not_observed" {
		t.Fatalf("missing input became zero: %+v", result)
	}
	result = project([]Pair{costPair("pair", 100, 50)}, "total_tokens", 0, false, measured("derived", 0, "tokens", "test input"))
	if result.Saving.Median != nil {
		t.Fatal("zero consumers yielded a valid scenario")
	}
}

func TestFullCueCostIsRequiredOnlyForComparableTotals(t *testing.T) {
	pair := costPair("pair", 100, 50)
	pair.Direct.Metrics["execution_duration_ms"] = measured("observed", 100, "ms", "agent")
	pair.Basic.Metrics["execution_duration_ms"] = measured("observed", 50, "ms", "agent")
	h := measured("derived", 20, "ms", "epoch")
	missingCue := project([]Pair{pair}, "execution_duration_ms", 2, false, h)
	if missingCue.Saving.Median != nil || comparePairs([]Pair{pair}, "execution_duration_ms").Difference.Median == nil {
		t.Fatal("missing cue cost was filled or erased agent-only comparison")
	}
	pair.Cue["execution_duration_ms"] = measured("observed", 10, "ms", "full CLI")
	present := project([]Pair{pair}, "execution_duration_ms", 2, false, h)
	if present.Saving.Median == nil || *present.Saving.Median != 60 {
		t.Fatalf("wrong combined duration: %+v", present)
	}
	pair.Cue["estimated_tokens"] = measured("estimated", 500, "tokens", "cue estimate")
	tokens := project([]Pair{pair}, "total_tokens", 1, false, measured("derived", 0, "tokens", "no LLM core"))
	if tokens.Saving.Median == nil || *tokens.Saving.Median != 50 {
		t.Fatal("cue estimate was added to observed agent tokens")
	}
	missingH := project([]Pair{pair}, "execution_duration_ms", 2, false, missing("ms", "epoch", "baseline unavailable"))
	if missingH.Saving.Median != nil {
		t.Fatal("missing baseline did not suppress combined duration")
	}
}

func TestConsumerScenariosAndArithmeticSafety(t *testing.T) {
	for _, test := range []struct {
		value int64
		want  []int64
		fail  bool
	}{
		{0, []int64{1, 2, 3, 5, 10}, false}, {5, []int64{1, 2, 3, 5, 10}, false}, {7, []int64{1, 2, 3, 5, 7, 10}, false}, {-1, nil, true}, {1 << 54, nil, true},
	} {
		got, err := consumerCounts(test.value)
		if (err != nil) != test.fail || !reflect.DeepEqual(got, test.want) {
			t.Fatalf("scenario %d: %v %v", test.value, got, err)
		}
	}
	if measured("estimated", math.Inf(1), "ms", "overflow").Value != nil {
		t.Fatal("non-finite measurement escaped")
	}
}

func TestEstimatedAgentMeasurementsAreExcludedFromArithmetic(t *testing.T) {
	pairs := []Pair{costPair("observed", 100, 90), costPair("estimated", 200, 1), costPair("derived", 300, 299)}
	value := pairs[1].Basic.Metrics["total_tokens"]
	value.Status = "estimated"
	pairs[1].Basic.Metrics["total_tokens"] = value
	value = pairs[2].Direct.Metrics["total_tokens"]
	value.Status = "derived"
	pairs[2].Direct.Metrics["total_tokens"] = value
	comparison := comparePairs(pairs, "total_tokens")
	if comparison.Difference.Count != 2 || comparison.Difference.Unknown != 1 || comparison.Difference.Median == nil || *comparison.Difference.Median != 5.5 {
		t.Fatal("source eligibility changed the observed/derived pair set")
	}
	if comparison.Pairs[1].Basic.Value != nil || comparison.Pairs[1].Basic.Status != "not_observed" || len(comparison.Pairs[1].Basic.Limitations) == 0 {
		t.Fatal("excluded estimate lacks an explicit unavailable reason")
	}
	projection := project(pairs, "total_tokens", 10, false, measured("derived", 0, "tokens", "no LLM core"))
	if projection.Saving.Count != 2 || projection.Saving.Unknown != 1 || projection.Saving.Median == nil || *projection.Saving.Median != 55 || projection.Pairs[1].BreakEven.Value != nil {
		t.Fatal("estimated source cost entered the projection")
	}
}
