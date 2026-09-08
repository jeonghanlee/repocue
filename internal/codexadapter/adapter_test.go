package codexadapter

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/jeonghanlee/repocue/internal/benchmark"
	"github.com/jeonghanlee/repocue/internal/evaluation"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestParseEventsUsesObservedUsageAndDerivedCommands(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stream := []byte(`{"type":"thread.started","thread_id":"test"}
{"type":"turn.started"}
{"type":"item.completed","item":{"type":"command_execution","command":"git status --short"}}
{"type":"item.completed","item":{"type":"command_execution","command":"sed -n '1,20p' README.md"}}
{"type":"item.completed","item":{"type":"agent_message","text":"{\"schema_version\":\"repocue/benchmark-answer-2\",\"project_purpose\":\"Example\",\"git\":{\"branch\":\"main\",\"head\":\"abc\",\"dirty\":false,\"tracked_changes\":[],\"untracked\":[]},\"primary_entry_points\":[],\"major_components\":[],\"important_documentation\":[],\"recent_relevant_changes\":[],\"project_symbols\":[],\"uncertainties\":[]}"}}
{"type":"turn.completed","usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10}}
{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":5,"output_tokens":5,"reasoning_output_tokens":2}}
`)
	observation, err := parseEvents(stream, Config{
		Condition: "ranked", RunIndex: 2, Repository: root, Snapshot: "snapshot-000001", Head: "abc",
		BenchmarkVersion: benchmark.Version, OutputSchemaVersion: benchmark.AnswerSchemaVersion,
		Model: "test-model", ReasoningEffort: "medium", Sandbox: "read-only",
	}, "codex-cli test", 0)
	if err != nil {
		t.Fatal(err)
	}
	if observation.Metrics.InputTokens == nil || *observation.Metrics.InputTokens != 130 {
		t.Fatalf("unexpected input tokens: %#v", observation.Metrics.InputTokens)
	}
	if observation.Metrics.TotalTokens == nil || *observation.Metrics.TotalTokens != 165 {
		t.Fatalf("unexpected total tokens: %#v", observation.Metrics.TotalTokens)
	}
	if observation.Metrics.GitCalls == nil || *observation.Metrics.GitCalls != 1 {
		t.Fatalf("unexpected Git count: %#v", observation.Metrics.GitCalls)
	}
	if observation.Metrics.RepositoryFilesNamed == nil || *observation.Metrics.RepositoryFilesNamed != 1 {
		t.Fatalf("unexpected file count: %#v", observation.Metrics.RepositoryFilesNamed)
	}
	if len(observation.UsageEvents) != 2 || observation.UsageEvents[0].InputTokens == nil || *observation.UsageEvents[0].InputTokens != 120 || observation.UsageEvents[1].InputTokens == nil || *observation.UsageEvents[1].InputTokens != 10 {
		t.Fatalf("unexpected usage events: %#v", observation.UsageEvents)
	}
	if len(observation.Commands) != 2 || len(observation.Commands[1].FilesRead) != 1 {
		t.Fatalf("unexpected command observations: %#v", observation.Commands)
	}
	validateRunnerSchema(t, observation)
}

func validateRunnerSchema(t *testing.T, observation any) {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	for _, name := range []string{"evaluation-runner-v4.schema.json", "benchmark-answer-v2.schema.json"} {
		serialized, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", name))
		if err != nil {
			t.Fatal(err)
		}
		var document any
		if err := json.Unmarshal(serialized, &document); err != nil {
			t.Fatal(err)
		}
		if err := compiler.AddResource("https://repocue.local/schema/"+name, document); err != nil {
			t.Fatal(err)
		}
	}
	schema, err := compiler.Compile("https://repocue.local/schema/evaluation-runner-v4.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(serialized, &document); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("runner schema validation failed: %v\n%s", err, serialized)
	}
}

func TestRunInvokesCodexAtOutermostBoundary(t *testing.T) {
	observation, err := runUsageFixture(t, `{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":1}}`)
	if err != nil {
		t.Fatal(err)
	}
	if observation.Metadata.CodexVersion != "codex-cli fixture" || observation.Condition != "direct" {
		t.Fatalf("unexpected observation: %#v", observation)
	}
}

func TestRunPreservesUsagePresence(t *testing.T) {
	pointer := func(value int64) *int64 { return &value }
	for _, test := range []struct {
		name   string
		stream string
		want   []*int64
		turns  int
	}{
		{"missing reasoning", `{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5}}`, []*int64{pointer(10), pointer(0), pointer(5), nil, pointer(15)}, 1},
		{"null reasoning", `{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":0,"output_tokens":5,"reasoning_output_tokens":null}}`, []*int64{pointer(10), pointer(0), pointer(5), nil, pointer(15)}, 1},
		{"explicit zeros", `{"type":"turn.completed","usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}`, []*int64{pointer(0), pointer(0), pointer(0), pointer(0), pointer(0)}, 1},
		{"absent usage", `{"type":"turn.completed"}`, []*int64{nil, nil, nil, nil, nil}, 1},
		{"null usage", `{"type":"turn.completed","usage":null}`, []*int64{nil, nil, nil, nil, nil}, 1},
		{"empty usage", `{"type":"turn.completed","usage":{}}`, []*int64{nil, nil, nil, nil, nil}, 1},
		{"partial turns", "{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}\n{\"type\":\"turn.completed\",\"usage\":{\"output_tokens\":2}}", []*int64{nil, nil, pointer(7), nil, nil}, 2},
		{"unobserved turn", "{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":10,\"output_tokens\":5}}\n{\"type\":\"turn.completed\"}", []*int64{nil, nil, nil, nil, nil}, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			observation, err := runUsageFixture(t, test.stream)
			if err != nil {
				t.Fatal(err)
			}
			m := observation.Metrics
			if !reflect.DeepEqual([]*int64{m.InputTokens, m.CachedInputTokens, m.OutputTokens, m.ReasoningOutputTokens, m.TotalTokens}, test.want) {
				t.Fatalf("unexpected token values: %+v", m)
			}
			if len(observation.UsageEvents) != test.turns || m.CommandExecutions == nil || *m.CommandExecutions != 0 || m.ExecutionDurationMS == nil {
				t.Fatal("missing usage erased completed turns or unrelated observations")
			}
			validateRunnerSchema(t, observation)
		})
	}
}

func TestRunRejectsInvalidUsageAtOutermostBoundary(t *testing.T) {
	for _, stream := range []string{
		"",
		`{"type":"turn.completed","usage":{"input_tokens":-1}}`,
		`{"type":"turn.completed","usage":{"input_tokens":1,"cached_input_tokens":2}}`,
		`{"type":"turn.completed","usage":{"input_tokens":9223372036854775807,"output_tokens":1}}`,
		`{"type":"turn.completed","usage":{"input_tokens":9223372036854775808}}`,
		`{"type":"turn.completed","usage":{"input_tokens":1.5}}`,
	} {
		t.Run(stream, func(t *testing.T) {
			if _, err := runUsageFixture(t, stream); err == nil {
				t.Fatal("invalid usage accepted")
			}
		})
	}
}

func runUsageFixture(t *testing.T, turns string) (evaluation.M2RunnerObservation, error) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the Codex fixture is a POSIX executable")
	}
	root := t.TempDir()
	fixture := filepath.Join(t.TempDir(), "codex")
	script := `#!/bin/sh
set -eu
if test "${1:-}" = "--version"; then
    printf '%s\n' 'codex-cli fixture'
    exit 0
fi
test "$1" = "exec"
cat >/dev/null
printf '%s\n' '{"type":"turn.started"}'
printf '%s\n' '{"type":"item.completed","item":{"type":"agent_message","text":"{\"schema_version\":\"repocue/benchmark-answer-2\",\"project_purpose\":\"Fixture\",\"git\":{\"branch\":null,\"head\":null,\"dirty\":false,\"tracked_changes\":[],\"untracked\":[]},\"primary_entry_points\":[],\"major_components\":[],\"important_documentation\":[],\"recent_relevant_changes\":[],\"project_symbols\":[],\"uncertainties\":[]}"}}'
`
	if turns != "" {
		script += "printf '%s\\n' '" + strings.ReplaceAll(turns, "'", "'\\''") + "'\n"
	}
	if err := os.WriteFile(fixture, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(t.TempDir(), "task.md")
	schema := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(task, []byte("Inspect the repository.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schema, []byte(`{"type":"object"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return Run(context.Background(), Config{
		Condition: "direct", RunIndex: 1, Repository: root, TaskFile: task, Snapshot: "snapshot-000001", Head: "abc",
		BenchmarkVersion: benchmark.Version, OutputSchemaVersion: benchmark.AnswerSchemaVersion,
		OutputSchemaFile: schema, CodexBinary: fixture, Model: "test-model",
		ReasoningEffort: "medium", Sandbox: "read-only",
	})
}

func TestRunReportsJSONLFailureWhenStderrIsEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Codex fixture is a POSIX executable")
	}
	root := t.TempDir()
	fixture := filepath.Join(t.TempDir(), "codex")
	script := `#!/bin/sh
set -eu
if test "${1:-}" = "--version"; then
    printf '%s\n' 'codex-cli fixture'
    exit 0
fi
cat >/dev/null
printf '%s\n' '{"type":"error","message":"invalid output schema"}'
exit 1
`
	if err := os.WriteFile(fixture, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(t.TempDir(), "task.md")
	schema := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(task, []byte("Inspect the repository.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schema, []byte(`{"type":"object"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Run(context.Background(), Config{
		Condition: "direct", RunIndex: 1, Repository: root, TaskFile: task, Snapshot: "snapshot-000001",
		BenchmarkVersion: benchmark.Version, OutputSchemaVersion: benchmark.AnswerSchemaVersion,
		OutputSchemaFile: schema, CodexBinary: fixture, Model: "test-model",
		ReasoningEffort: "medium", Sandbox: "read-only",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid output schema") {
		t.Fatalf("unexpected error: %v", err)
	}
}
