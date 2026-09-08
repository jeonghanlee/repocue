package evaluation

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/jeonghanlee/repocue/docs/schema"
	"github.com/jeonghanlee/repocue/internal/model"
)

func tokenPointer(value int64) *int64 { return &value }

func TestTokenMetricsFromUsage(t *testing.T) {
	for _, test := range []struct {
		name   string
		events string
		want   []*int64
	}{
		{"zero", `[{"turn":1,"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}]`, []*int64{tokenPointer(0), tokenPointer(0), tokenPointer(0), tokenPointer(0), tokenPointer(0)}},
		{"omitted subsets", `[{"turn":1,"input_tokens":10,"output_tokens":2}]`, []*int64{tokenPointer(10), nil, tokenPointer(2), nil, tokenPointer(12)}},
		{"null subsets", `[{"turn":1,"input_tokens":10,"cached_input_tokens":null,"output_tokens":2,"reasoning_output_tokens":null}]`, []*int64{tokenPointer(10), nil, tokenPointer(2), nil, tokenPointer(12)}},
		{"partial input", `[{"turn":1,"input_tokens":10,"output_tokens":2},{"turn":2,"output_tokens":3}]`, []*int64{nil, nil, tokenPointer(5), nil, nil}},
		{"partial output", `[{"turn":1,"input_tokens":10,"output_tokens":2},{"turn":2,"input_tokens":3}]`, []*int64{tokenPointer(13), nil, nil, nil, nil}},
		{"no usage", `[{"turn":1}]`, []*int64{nil, nil, nil, nil, nil}},
		{"unobserved turn", `[{"turn":1,"input_tokens":10,"output_tokens":2},{"turn":2}]`, []*int64{nil, nil, nil, nil, nil}},
		{"multiple complete", `[{"turn":1,"input_tokens":10,"cached_input_tokens":4,"output_tokens":2,"reasoning_output_tokens":1},{"turn":2,"input_tokens":20,"cached_input_tokens":5,"output_tokens":3,"reasoning_output_tokens":1}]`, []*int64{tokenPointer(30), tokenPointer(9), tokenPointer(5), tokenPointer(2), tokenPointer(35)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var events []UsageEvent
			if err := json.Unmarshal([]byte(test.events), &events); err != nil {
				t.Fatal(err)
			}
			got, err := TokenMetricsFromUsage(events)
			if err != nil {
				t.Fatal(err)
			}
			actual := []*int64{got.InputTokens, got.CachedInputTokens, got.OutputTokens, got.ReasoningOutputTokens, got.TotalTokens}
			if !reflect.DeepEqual(actual, test.want) {
				t.Fatalf("metrics: %+v, want %+v", got, test.want)
			}
			for name, value := range tokenValues(got) {
				want := measurementObserved
				if value == nil {
					want = measurementUnobserved
				} else if name == "total_tokens" {
					want = measurementDerived
				}
				if got.Statuses[name] != want {
					t.Fatalf("%s status: %s", name, got.Statuses[name])
				}
			}
		})
	}
}

func TestTokenMetricsRejectInvalidUsage(t *testing.T) {
	for _, test := range []struct {
		name   string
		events []UsageEvent
	}{
		{"no turns", nil},
		{"bad order", []UsageEvent{{Turn: 2}}},
		{"negative input", []UsageEvent{{Turn: 1, InputTokens: tokenPointer(-1)}}},
		{"negative cached", []UsageEvent{{Turn: 1, CachedInputTokens: tokenPointer(-1)}}},
		{"negative output", []UsageEvent{{Turn: 1, OutputTokens: tokenPointer(-1)}}},
		{"negative reasoning", []UsageEvent{{Turn: 1, ReasoningOutputTokens: tokenPointer(-1)}}},
		{"cached exceeds input", []UsageEvent{{Turn: 1, InputTokens: tokenPointer(1), CachedInputTokens: tokenPointer(2)}}},
		{"input overflow", []UsageEvent{{Turn: 1, InputTokens: tokenPointer(math.MaxInt64)}, {Turn: 2, InputTokens: tokenPointer(1)}}},
		{"output overflow", []UsageEvent{{Turn: 1, OutputTokens: tokenPointer(math.MaxInt64)}, {Turn: 2, OutputTokens: tokenPointer(1)}}},
		{"cached overflow", []UsageEvent{{Turn: 1, CachedInputTokens: tokenPointer(math.MaxInt64)}, {Turn: 2, CachedInputTokens: tokenPointer(1)}}},
		{"reasoning overflow", []UsageEvent{{Turn: 1, ReasoningOutputTokens: tokenPointer(math.MaxInt64)}, {Turn: 2, ReasoningOutputTokens: tokenPointer(1)}}},
		{"total overflow", []UsageEvent{{Turn: 1, InputTokens: tokenPointer(math.MaxInt64), OutputTokens: tokenPointer(1)}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := TokenMetricsFromUsage(test.events); err == nil {
				t.Fatal("invalid usage accepted")
			}
		})
	}
}

func TestCalibrationReadsOriginalVersion3Report(t *testing.T) {
	// Captured through prepareReportingFixture at a8f650c before the v4 edit.
	// The digest pins actual old CLI output, not relabeled current output.
	data, err := os.ReadFile("testdata/evaluation-v3.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "681b676c0a0d12cb14237ed530fd9ccf32bc9353867affd7f0a973fe78b1c463" {
		t.Fatalf("legacy report bytes changed: %s", got)
	}
	for _, test := range []struct {
		name   string
		mutate func(*M2ConditionReport)
		valid  bool
	}{
		{"original", func(*M2ConditionReport) {}, true},
		{"missing legacy counter", func(r *M2ConditionReport) { r.Runner.UsageEvents[0].ReasoningOutputTokens = nil }, false},
		{"missing legacy aggregate", func(r *M2ConditionReport) {
			r.Runner.Metrics.ReasoningOutputTokens = nil
			r.Runner.Metrics.Statuses["reasoning_output_tokens"] = measurementUnobserved
		}, false},
		{"wrong runner version", func(r *M2ConditionReport) { r.Runner.SchemaVersion = M2RunnerSchemaVersion }, false},
		{"wrong report version", func(r *M2ConditionReport) { r.SchemaVersion = M2SchemaVersion }, false},
		{"unknown report version", func(r *M2ConditionReport) { r.SchemaVersion = "repocue/evaluation-999" }, false},
		{"unknown runner version", func(r *M2ConditionReport) { r.Runner.SchemaVersion = "repocue/evaluation-runner-999" }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var report M2ConditionReport
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatal(err)
			}
			test.mutate(&report)
			err := ValidateCalibrationReport(report, model.Basis{Branch: report.Repository.Branch, Head: report.Repository.Head, Dirty: report.Repository.Dirty})
			if (err == nil) != test.valid {
				t.Fatalf("runtime validity: %v", err)
			}
			schemaName, err := CalibrationReportSchema(report.SchemaVersion)
			if err == nil {
				encoded, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				err = schema.Validate(schemaName, encoded)
				if (err == nil) != test.valid {
					t.Fatalf("schema validity: %v", err)
				}
			} else if test.valid {
				t.Fatal(err)
			}
		})
	}
}
