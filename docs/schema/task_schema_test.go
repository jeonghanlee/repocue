package schema

import "testing"

func TestTaskContractsStrictVersionsAndUnknownValues(t *testing.T) {
	for _, name := range []string{"task-context-v1.schema.json", "task-cue-v1.schema.json",
		"task-benchmark-v1.schema.json", "task-answer-v1.schema.json", "task-runner-v1.schema.json", "task-evaluation-v1.schema.json"} {
		if err := Validate(name, []byte("{}")); err == nil {
			t.Fatalf("incomplete public contract accepted: %s", name)
		}
	}
	for _, test := range []struct {
		value string
		valid bool
	}{
		{`{"status":"not_observed","value":null,"unit":"credits","reason":"missing"}`, true},
		{`{"status":"observed","value":0,"unit":"credits","reason":"measured zero"}`, true},
		{`{"status":"not_observed","value":0,"unit":"credits","reason":"missing"}`, false},
		{`{"status":"observed","value":null,"unit":"credits","reason":"missing"}`, false},
		{`{"status":"invented","value":0,"unit":"credits","reason":"unsupported"}`, false},
		{`{"status":"observed","value":0,"unit":"credits","reason":"measured","extra":true}`, false},
	} {
		if err := Validate("task-evaluation-v1.schema.json#/$defs/metric", []byte(test.value)); (err == nil) != test.valid {
			t.Fatalf("metric presence contract: %v %s", err, test.value)
		}
	}
}
