package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jeonghanlee/repocue/internal/benchmark"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const m2RunnerObservationScript = `printf '{"schema_version":"repocue/evaluation-runner-3","condition":"%s","run_index":%s,"metadata":{"adapter":"fixture","adapter_version":"1","model":"fixture","reasoning_effort":"medium","sandbox":"read-only","benchmark_version":"repository-state-v2","output_schema_version":"repocue/benchmark-answer-2","repository_head":"%s","repocue_snapshot":"%s"},"metrics":{"input_tokens":1,"cached_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0,"total_tokens":2,"statuses":{"input_tokens":"observed","cached_input_tokens":"observed","output_tokens":"observed","reasoning_output_tokens":"observed","total_tokens":"derived"}},"usage_events":[{"turn":1,"input_tokens":1,"cached_input_tokens":0,"output_tokens":1,"reasoning_output_tokens":0}],"tokenizer_counts":[],"commands":[],"final_response":{"schema_version":"repocue/benchmark-answer-2","project_purpose":"Fixture","git":{"branch":"main","head":"%s","dirty":false,"tracked_changes":[],"untracked":[]},"primary_entry_points":[],"major_components":[],"important_documentation":[],"recent_relevant_changes":[],"project_symbols":[],"uncertainties":[]},"findings":[],"limitations":[]}\n' "$REPOCUE_EVAL_CONDITION" "$REPOCUE_EVAL_RUN_INDEX" "$REPOCUE_EVAL_HEAD" "$REPOCUE_EVAL_SNAPSHOT" "$REPOCUE_EVAL_HEAD"
`

func TestConditionSetDryRunUsesRealOracleAndWritesAtomicReports(t *testing.T) {
	root := createM2Repository(t)
	reports := t.TempDir()
	temporaryRoot := filepath.Join(t.TempDir(), "m2-work")
	manifest, err := RunM2(context.Background(), M2Config{
		RepositoryPath: root, MaxTokens: 500, OracleTool: realOraclePath(t),
		OutputDirectory: reports, RunIndex: 1, TemporaryRoot: temporaryRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(manifest.ConditionOrder, M2Conditions) || len(manifest.Reports) != len(M2Conditions) {
		t.Fatalf("unexpected condition set: %#v", manifest)
	}
	entries, err := os.ReadDir(reports)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("got report-set entries %#v, want one directory", entries)
	}
	if info, err := os.Stat(temporaryRoot); err != nil || !info.IsDir() {
		t.Fatalf("temporary root was not created: info=%#v err=%v", info, err)
	}
	reportEntries, err := os.ReadDir(filepath.Join(reports, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if len(reportEntries) != len(M2Conditions) {
		t.Fatalf("got %d reports, want %d", len(reportEntries), len(M2Conditions))
	}
	schema := compileM2Schema(t, "evaluation-v3.schema.json")
	for index, report := range manifest.Reports {
		if report.Condition != M2Conditions[index] || report.ConditionOrder != index+1 {
			t.Fatalf("unexpected report order: %#v", report)
		}
		if report.MaintenanceID != manifest.MaintenanceID || report.RunIndex != 1 {
			t.Fatalf("report lost shared maintenance identity: %#v", report)
		}
		if report.Cue.EstimatedTokens > 500 {
			t.Fatalf("condition %s exceeded its budget: %#v", report.Condition, report.Cue)
		}
		if report.Condition == ConditionStructuralOracle &&
			(report.Cue.StructuralAnalysisDurationMS <= 0 || report.Cue.StructuralCandidateCount == 0) {
			t.Fatalf("structural cost was not measured: %#v", report.Cue)
		}
		validateM2Schema(t, schema, report)
	}
	for _, entry := range append(entries, reportEntries...) {
		if strings.Contains(entry.Name(), ".draft-") {
			t.Fatalf("completed run exposed a draft report: %s", entry.Name())
		}
	}
}

func TestConditionRunnerStartsFreshProcessForEveryCondition(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the external runner fixture is a POSIX executable")
	}
	root := createM2Repository(t)
	workspace := t.TempDir()
	logPath := filepath.Join(workspace, "conditions.log")
	runner := filepath.Join(workspace, "runner")
	script := `#!/bin/sh
set -eu
printf '%s\n' "$REPOCUE_EVAL_CONDITION" >>"$REPOCUE_TEST_LOG"
` + m2RunnerObservationScript
	if err := os.WriteFile(runner, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(workspace, "task.md")
	if err := os.WriteFile(task, []byte("Inspect the repository.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REPOCUE_TEST_LOG", logPath)
	manifest, err := RunM2(context.Background(), M2Config{
		RepositoryPath: root, MaxTokens: 500, TaskFile: task, Runner: runner,
		OracleTool: realOraclePath(t), RunIndex: 3, TemporaryRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Fields(string(serialized))
	if !slices.Equal(got, M2Conditions) {
		t.Fatalf("runner conditions = %#v, want %#v", got, M2Conditions)
	}
	for _, report := range manifest.Reports {
		if report.Runner == nil || report.Runner.Condition != report.Condition || report.Runner.RunIndex != 3 {
			t.Fatalf("runner observation mismatch: %#v", report)
		}
		validateM2Schema(t, compileM2Schema(t, "evaluation-v3.schema.json"), report)
	}
}

func TestConditionSetFailurePublishesNothingAndCanRetry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the external runner fixture is a POSIX executable")
	}
	root := createM2Repository(t)
	workspace := t.TempDir()
	reports := filepath.Join(workspace, "reports")
	runner := filepath.Join(workspace, "runner")
	script := `#!/bin/sh
set -eu
if [ "${REPOCUE_TEST_FAIL_CONDITION:-}" = "$REPOCUE_EVAL_CONDITION" ]; then
    exit 42
fi
` + m2RunnerObservationScript
	if err := os.WriteFile(runner, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(workspace, "task.md")
	if err := os.WriteFile(task, []byte("Inspect the repository.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := M2Config{
		RepositoryPath: root, MaxTokens: 500, TaskFile: task, Runner: runner,
		OracleTool: realOraclePath(t), OutputDirectory: reports, RunIndex: 1,
	}
	t.Setenv("REPOCUE_TEST_FAIL_CONDITION", ConditionRanked)
	if _, err := RunM2(context.Background(), config); err == nil {
		t.Fatal("failed condition set was accepted")
	}
	if entries, err := os.ReadDir(reports); err == nil && len(entries) != 0 {
		t.Fatalf("failed condition set exposed output: %#v", entries)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	t.Setenv("REPOCUE_TEST_FAIL_CONDITION", "")
	if _, err := RunM2(context.Background(), config); err != nil {
		t.Fatalf("retry after failed condition set: %v", err)
	}
}

func TestM2RejectsRepositoryInternalOutputAndTemporaryPaths(t *testing.T) {
	root := createM2Repository(t)
	for _, test := range []struct {
		name   string
		config M2Config
	}{
		{"output", M2Config{OutputDirectory: filepath.Join(root, "reports")}},
		{"temporary", M2Config{TemporaryRoot: filepath.Join(root, "temporary")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := test.config
			config.RepositoryPath = root
			config.MaxTokens = 500
			config.OracleTool = realOraclePath(t)
			config.RunIndex = 1
			if _, err := RunM2(context.Background(), config); err == nil || !strings.Contains(err.Error(), "outside the evaluated repository") {
				t.Fatalf("repository-internal path was accepted: %v", err)
			}
		})
	}
	if runtime.GOOS != "windows" {
		linkedRoot := filepath.Join(t.TempDir(), "repository-link")
		if err := os.Symlink(root, linkedRoot); err != nil {
			t.Fatal(err)
		}
		_, err := RunM2(context.Background(), M2Config{
			RepositoryPath: root, MaxTokens: 500, OracleTool: realOraclePath(t),
			OutputDirectory: filepath.Join(linkedRoot, "reports"), RunIndex: 1,
		})
		if err == nil || !strings.Contains(err.Error(), "outside the evaluated repository") {
			t.Fatalf("repository-internal symlink path was accepted: %v", err)
		}
	}
}

func TestM2RejectsRepositoryChangeDuringStructuralContextPreparation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the structural oracle fixture is a POSIX executable")
	}
	root := createM2Repository(t)
	workspace := t.TempDir()
	oracle := filepath.Join(workspace, "oracle")
	script := `#!/bin/sh
set -eu
printf '\nchanged during oracle\n' >>"$1/README.md"
printf 'Python\tsrc/service.py\tclass\tService\tclass Service\n'
`
	if err := os.WriteFile(oracle, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	reports := filepath.Join(workspace, "reports")
	_, err := RunM2(context.Background(), M2Config{
		RepositoryPath: root, MaxTokens: 500, OracleTool: oracle,
		OutputDirectory: reports, RunIndex: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "structural context basis") {
		t.Fatalf("changed structural context basis was accepted: %v", err)
	}
	if entries, readErr := os.ReadDir(reports); readErr == nil && len(entries) != 0 {
		t.Fatalf("failed context preparation exposed reports: %#v", entries)
	} else if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
}

func TestAtomicReportRejectsInvalidRunBeforeWriting(t *testing.T) {
	reports := t.TempDir()
	_, err := RunM2(context.Background(), M2Config{
		RepositoryPath: createM2Repository(t), MaxTokens: 500,
		OracleTool: realOraclePath(t), OutputDirectory: reports, RunIndex: 0,
	})
	if err == nil {
		t.Fatal("invalid run index was accepted")
	}
	entries, readErr := os.ReadDir(reports)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("invalid run exposed reports: %#v", entries)
	}
}

func TestM2RunnerObservationValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*M2RunnerObservation)
	}{
		{"negative metric", func(value *M2RunnerObservation) { *value.Metrics.InputTokens = -1 }},
		{"invalid status", func(value *M2RunnerObservation) {
			value.Metrics.Statuses = map[string]string{"input_tokens": "guessed"}
		}},
		{"status without value", func(value *M2RunnerObservation) { value.Metrics.Statuses = map[string]string{"git_calls": "observed"} }},
		{"inconsistent total", func(value *M2RunnerObservation) { *value.Metrics.TotalTokens = 99 }},
		{"inconsistent usage", func(value *M2RunnerObservation) { value.UsageEvents[0].OutputTokens = 3 }},
		{"wrong snapshot", func(value *M2RunnerObservation) { value.Metadata.RepoCueSnapshot = "snapshot-wrong" }},
		{"missing final response", func(value *M2RunnerObservation) { value.FinalResponse = nil }},
		{"invalid final response schema", func(value *M2RunnerObservation) { value.FinalResponse = json.RawMessage(`{}`) }},
		{"metric without status", func(value *M2RunnerObservation) { delete(value.Metrics.Statuses, "input_tokens") }},
		{"direct fallback metric", func(value *M2RunnerObservation) { count := int64(0); value.Metrics.FallbackRepositoryCommands = &count }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation, expected := validM2Observation()
			test.mutate(&observation)
			if err := validateM2Observation(observation, ConditionDirect, 1, expected); err == nil {
				t.Fatal("invalid runner observation was accepted")
			}
		})
	}

	observation, expected := validM2Observation()
	if err := validateM2Observation(observation, ConditionDirect, 1, expected); err != nil {
		t.Fatalf("valid runner observation was rejected: %v", err)
	}
	validateM2Schema(t, compileM2Schema(t, "evaluation-runner-v3.schema.json"), observation)
}

func TestM2RunnerIdentityRequiresIdenticalConditions(t *testing.T) {
	left, _ := validM2Observation()
	right := left.Metadata
	right.Model = "different"
	if err := validateM2RunnerIdentity(left.Metadata, right); err == nil {
		t.Fatal("different runner identities were accepted")
	}
}

func TestM2RunnerSchemaRejectsMissingStatusAndInvalidAnswer(t *testing.T) {
	schema := compileM2Schema(t, "evaluation-runner-v3.schema.json")
	tests := []struct {
		name   string
		mutate func(*M2RunnerObservation)
	}{
		{"missing metric status", func(value *M2RunnerObservation) { delete(value.Metrics.Statuses, "input_tokens") }},
		{"invalid benchmark answer", func(value *M2RunnerObservation) { value.FinalResponse = json.RawMessage(`{}`) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation, _ := validM2Observation()
			test.mutate(&observation)
			serialized, err := json.Marshal(observation)
			if err != nil {
				t.Fatal(err)
			}
			var document any
			if err := json.Unmarshal(serialized, &document); err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(document); err == nil {
				t.Fatal("invalid runner observation satisfied the JSON schema")
			}
		})
	}
}

func validM2Observation() (M2RunnerObservation, model.Snapshot) {
	head := "0123456789abcdef"
	expected := model.Snapshot{ID: "snapshot-000001", Basis: model.Basis{Head: &head}}
	input := int64(10)
	cached := int64(4)
	output := int64(2)
	reasoning := int64(1)
	total := int64(12)
	return M2RunnerObservation{
		SchemaVersion: M2RunnerSchemaVersion, Condition: ConditionDirect, RunIndex: 1,
		Metadata: RunnerMetadata{
			Adapter: "fixture", AdapterVersion: "1", Model: "fixture", ReasoningEffort: "medium", Sandbox: "read-only",
			BenchmarkVersion: benchmark.Version, OutputSchemaVersion: benchmark.AnswerSchemaVersion,
			RepositoryHead: head, RepoCueSnapshot: expected.ID,
		},
		Metrics: M2RunnerMetrics{
			InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &output,
			ReasoningOutputTokens: &reasoning, TotalTokens: &total,
			Statuses: map[string]string{
				"input_tokens": "observed", "cached_input_tokens": "observed", "output_tokens": "observed",
				"reasoning_output_tokens": "observed", "total_tokens": "derived",
			},
		},
		UsageEvents: []UsageEvent{{
			Turn: 1, InputTokens: input, CachedInputTokens: cached,
			OutputTokens: output, ReasoningOutputTokens: reasoning,
		}},
		TokenizerCounts: []TokenizerCount{}, Commands: []CommandObservation{},
		FinalResponse: validBenchmarkResponse(head), Findings: []ContextFinding{}, Limitations: []string{},
	}, expected
}

func validBenchmarkResponse(head string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"schema_version":"repocue/benchmark-answer-2","project_purpose":"Fixture","git":{"branch":null,"head":%q,"dirty":false,"tracked_changes":[],"untracked":[]},"primary_entry_points":[],"major_components":[],"important_documentation":[],"recent_relevant_changes":[],"project_symbols":[],"uncertainties":[]}`, head))
}

func TestStructuralOracleRealTool(t *testing.T) {
	root := createM2Repository(t)
	candidates, limitations, err := runStructuralOracle(context.Background(), realOraclePath(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(limitations) != 0 {
		t.Fatalf("unexpected limitations: %#v", limitations)
	}
	var sawPythonClass, sawPythonMethod, sawBashFunction, sawBashDependency bool
	for _, candidate := range candidates {
		switch {
		case candidate.Language == "Python" && candidate.Kind == "class" && candidate.Name == "Service":
			sawPythonClass = true
		case candidate.Language == "Python" && candidate.Kind == "method" && candidate.Name == "run":
			sawPythonMethod = true
		case candidate.Language == "Bash" && candidate.Kind == "function" && candidate.Name == "main":
			sawBashFunction = true
		case candidate.Language == "Bash" && candidate.Kind == "dependency":
			sawBashDependency = true
		}
	}
	if !sawPythonClass || !sawPythonMethod || !sawBashFunction || !sawBashDependency {
		t.Fatalf("oracle candidates are incomplete: %#v", candidates)
	}
}

func createM2Repository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.name", "RepoCue Test")
	runGit(t, root, "config", "user.email", "repocue@example.invalid")
	files := map[string]string{
		"README.md":             "# M2 Fixture\n",
		"src/service.py":        "import os\n\nclass Service:\n    def run(self, value: str) -> str:\n        return value\n",
		"tests/test_service.py": "from src.service import Service\n\ndef test_run():\n    assert Service().run('x') == 'x'\n",
		"bin/run.bash":          "#!/usr/bin/env bash\nsource ../lib/common.bash\nfunction main {\n    printf '%s\\n' test\n}\n",
		"lib/common.bash":       "helper() {\n    :\n}\n",
	}
	for path, content := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "Initialize M2 fixture")
	return root
}

func realOraclePath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "tools", "evaluation", "structural-oracle.bash"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func compileM2Schema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	seen := map[string]bool{}
	for _, resourceName := range []string{name, "evaluation-runner-v3.schema.json", "benchmark-answer-v2.schema.json"} {
		if seen[resourceName] {
			continue
		}
		seen[resourceName] = true
		serialized, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", resourceName))
		if err != nil {
			t.Fatal(err)
		}
		var document any
		if err := json.Unmarshal(serialized, &document); err != nil {
			t.Fatal(err)
		}
		if err := compiler.AddResource("https://repocue.local/schema/"+resourceName, document); err != nil {
			t.Fatal(err)
		}
	}
	schema, err := compiler.Compile("https://repocue.local/schema/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func validateM2Schema(t *testing.T, schema *jsonschema.Schema, value any) {
	t.Helper()
	serialized, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(serialized, &document); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("schema validation failed: %v\n%s", err, serialized)
	}
}
