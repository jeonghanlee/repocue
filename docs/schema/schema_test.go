package schema

import (
	"encoding/json"
	"testing"
)

func TestVersion4TokenPresenceContract(t *testing.T) {
	for _, name := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens"} {
		for _, test := range []struct {
			name    string
			value   any
			present bool
			status  string
			valid   bool
		}{
			{"omitted", nil, false, "not_observed", true},
			{"null", nil, true, "not_observed", true},
			{"zero", int64(0), true, "observed", true},
			{"estimated", int64(1), true, "estimated", true},
			{"unknown zero", int64(0), true, "not_observed", false},
			{"observed null", nil, true, "observed", false},
			{"observed absent", nil, false, "observed", false},
			{"negative", int64(-1), true, "observed", false},
		} {
			t.Run(name+"/"+test.name, func(t *testing.T) {
				statuses := map[string]string{"input_tokens": "not_observed", "cached_input_tokens": "not_observed", "output_tokens": "not_observed", "reasoning_output_tokens": "not_observed", "total_tokens": "not_observed"}
				statuses[name] = test.status
				metrics := map[string]any{"statuses": statuses}
				if test.present {
					metrics[name] = test.value
				}
				data, err := json.Marshal(metrics)
				if err != nil {
					t.Fatal(err)
				}
				err = Validate("evaluation-runner-v4.schema.json#/properties/metrics", data)
				if (err == nil) != test.valid {
					t.Fatalf("metric validity: %v %s", err, data)
				}
			})
		}
	}
	for _, data := range []string{`{"turn":1}`, `{"turn":1,"input_tokens":null}`, `{"turn":1,"input_tokens":0}`} {
		if err := Validate("evaluation-runner-v4.schema.json#/properties/usage_events/items", []byte(data)); err != nil {
			t.Fatal(err)
		}
		if err := Validate("evaluation-runner-v3.schema.json#/properties/usage_events/items", []byte(data)); err == nil {
			t.Fatal("incomplete legacy usage accepted")
		}
	}
}

func TestMeasurementContractPreservesUnknownAndMeasuredZero(t *testing.T) {
	for _, test := range []struct {
		input string
		valid bool
	}{
		{`{"status":"not_observed","value":null,"unit":"ms","basis":"missing","limitations":[]}`, true},
		{`{"status":"observed","value":0,"unit":"ms","basis":"process observation","limitations":[]}`, true},
		{`{"status":"not_observed","value":0,"unit":"ms","basis":"missing","limitations":[]}`, false},
		{`{"status":"observed","value":null,"unit":"ms","basis":"missing","limitations":[]}`, false},
		{`{"status":"invented","value":0,"unit":"ms","basis":"missing","limitations":[]}`, false},
	} {
		if err := Validate("calibration-v1.schema.json#/$defs/measurement", []byte(test.input)); (err == nil) != test.valid {
			t.Fatalf("measurement validity: %v %s", err, test.input)
		}
	}
}

func TestEmbeddedContractSelectionDoesNotLoadExternalSchemas(t *testing.T) {
	for _, name := range []string{"../../absent.schema.json", "https://example.invalid/schema.json", "missing.schema.json"} {
		if err := Validate(name, []byte(`{}`)); err == nil {
			t.Fatalf("untrusted schema accepted: %s", name)
		}
	}
}
