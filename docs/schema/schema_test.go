package schema

import "testing"

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
