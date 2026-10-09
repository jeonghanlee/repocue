package taskbenchmark

import (
	"encoding/json"
	"testing"

	"github.com/jeonghanlee/repocue/internal/cue"
)

func TestTaskBenchmarkPolicyAndVersionIsolation(t *testing.T) {
	spec := Spec{SchemaVersion: Version, ID: "check", TaskKind: "verification", ContextFile: "docs/context.json",
		Targets: []string{"cmd"}, MaxTokens: 5000, Prompt: "Run required checks.",
		Policy: &Policy{ID: "policy", RequiredPairs: 1, Rubric: "Complete original checks.", Checks: []Check{}}}
	serialized, _ := json.Marshal(spec)
	if _, err := Decode(serialized); err == nil {
		t.Fatal("empty verification policy accepted")
	}
	spec.Policy.Checks = []Check{{ID: "check", Command: "go test ./...", OutputContains: []string{}}}
	serialized, _ = json.Marshal(spec)
	if _, err := Decode(serialized); err != nil {
		t.Fatal(err)
	}
	spec.Policy.Checks = append(spec.Policy.Checks, spec.Policy.Checks[0])
	serialized, _ = json.Marshal(spec)
	if _, err := Decode(serialized); err == nil {
		t.Fatal("duplicate frozen check accepted")
	}
	spec.Policy = nil
	missing, _, err := Score(spec, cue.TaskEnvelope{}, Transport{Status: "completed"}, nil, "")
	if err != nil || missing.Value != nil {
		t.Fatal("missing policy inferred successful quality")
	}
	spec.SchemaVersion = "repocue/evaluation-3"
	serialized, _ = json.Marshal(spec)
	if _, err := Decode(serialized); err == nil {
		t.Fatal("legacy contract entered task scorer")
	}
	if _, err := OutputSchema(); err != nil {
		t.Fatal(err)
	}
}
