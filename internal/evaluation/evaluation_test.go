package evaluation

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jeonghanlee/repocue/internal/benchmark"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestRepositoryProbeUsesRealGitRepository(t *testing.T) {
	root := createRepository(t)
	before := gitStatus(t, root)
	report, err := Run(context.Background(), Config{
		RepositoryPath: root,
		MaxTokens:      500,
		TemporaryRoot:  t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Repository.TrackedFiles != 2 || report.Baseline.FilesScanned != 2 {
		t.Fatalf("unexpected baseline counts: %#v", report.Baseline)
	}
	if report.Baseline.BytesScanned == 0 || report.Baseline.GitCommands == 0 {
		t.Fatalf("baseline did not record repository costs: %#v", report.Baseline)
	}
	if report.Update.Changed || report.Update.Kind != "no-op" {
		t.Fatalf("unexpected update probe: %#v", report.Update)
	}
	if report.Cue.OutputBytes == 0 || report.Cue.EstimatedTokens > report.Cue.MaxTokens {
		t.Fatalf("unexpected cue measurement: %#v", report.Cue)
	}
	if report.Direct.Status != measurementUnobserved || report.Assisted.Status != measurementUnobserved {
		t.Fatalf("runner metrics were not marked unobserved: %#v %#v", report.Direct, report.Assisted)
	}
	if len(report.Reuse) != 5 || report.Reuse[4].Consumers != 10 {
		t.Fatalf("unexpected reuse projections: %#v", report.Reuse)
	}
	if after := gitStatus(t, root); after != before {
		t.Fatalf("evaluation modified repository status: before=%q after=%q", before, after)
	}
	validateSchema(t, "evaluation-v2.schema.json", report)
}

func TestExternalRunnerContractAndComparison(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the runner fixture is a POSIX executable")
	}
	root := createRepository(t)
	workspace := t.TempDir()
	runner := filepath.Join(workspace, "runner")
	runnerSource := `#!/bin/sh
set -eu
test "$REPOCUE_EVAL_SCHEMA" = "repocue/evaluation-runner-2"
test -f "$REPOCUE_EVAL_TASK_FILE"
if test "$REPOCUE_EVAL_ARM" = "assisted"; then
    test -f "$REPOCUE_EVAL_CUE_FILE"
    printf '%s\n' '{"schema_version":"repocue/evaluation-runner-2","arm":"assisted","metadata":{"adapter":"fixture","adapter_version":"1","codex_version":"fixture","model":"fixture","reasoning_effort":"medium","sandbox":"read-only","benchmark_version":"repository-state-v1","output_schema_version":"repocue/benchmark-answer-1","repository_head":"'$REPOCUE_EVAL_HEAD'","repocue_snapshot":"'$REPOCUE_EVAL_SNAPSHOT'"},"metrics":{"input_tokens":40,"cached_input_tokens":5,"output_tokens":10,"reasoning_output_tokens":2,"total_tokens":50,"execution_duration_ms":20,"repository_files_read":2,"repository_bytes_read":100,"git_calls":1,"filesystem_search_calls":1,"tool_calls":3,"fallback_repository_commands":2,"fallback_repository_files_read":2,"fallback_repository_bytes_read":100},"tokenizer_counts":[{"tokenizer":"fixture","tokens":38}],"commands":[],"findings":[]}'
else
    test -z "$REPOCUE_EVAL_CUE_FILE"
    printf '%s\n' '{"schema_version":"repocue/evaluation-runner-2","arm":"direct","metadata":{"adapter":"fixture","adapter_version":"1","codex_version":"fixture","model":"fixture","reasoning_effort":"medium","sandbox":"read-only","benchmark_version":"repository-state-v1","output_schema_version":"repocue/benchmark-answer-1","repository_head":"'$REPOCUE_EVAL_HEAD'","repocue_snapshot":"'$REPOCUE_EVAL_SNAPSHOT'"},"metrics":{"input_tokens":100,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":4,"total_tokens":120,"execution_duration_ms":50,"repository_files_read":8,"repository_bytes_read":1000,"git_calls":4,"filesystem_search_calls":3,"tool_calls":10},"tokenizer_counts":[{"tokenizer":"fixture","tokens":95}],"commands":[],"findings":[]}'
fi
`
	if err := os.WriteFile(runner, []byte(runnerSource), 0o700); err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(workspace, "task.txt")
	if err := os.WriteFile(task, []byte("Describe the repository state.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), Config{
		RepositoryPath: root,
		MaxTokens:      500,
		TaskFile:       task,
		DirectRunner:   runner,
		AssistedRunner: runner,
		TemporaryRoot:  workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Direct.Status != measurementObserved || report.Assisted.Status != measurementObserved {
		t.Fatalf("runner observations were not recorded: %#v %#v", report.Direct, report.Assisted)
	}
	if value := report.Comparison.InputTokenReduction.Value; value == nil || *value != 60 {
		t.Fatalf("unexpected token comparison: %#v", report.Comparison.InputTokenReduction)
	}
	if value := report.Comparison.ToolCallReduction.Value; value == nil || *value != 7 {
		t.Fatalf("unexpected tool-call comparison: %#v", report.Comparison.ToolCallReduction)
	}
	projection := report.Reuse[4]
	if value := projection.DirectTotalTokens.Value; value == nil || *value != 1200 {
		t.Fatalf("unexpected direct reuse projection: %#v", projection)
	}
	if value := projection.AssistedTotalTokens.Value; value == nil || *value != 500 {
		t.Fatalf("unexpected assisted reuse projection: %#v", projection)
	}
	observation := RunnerObservation{
		SchemaVersion: RunnerSchemaVersion,
		Arm:           "direct",
		Metadata:      RunnerMetadata{Adapter: "fixture", AdapterVersion: "1"},
		Metrics:       RunnerMetrics{},
	}
	validateSchema(t, "evaluation-runner-v2.schema.json", observation)
}

func TestProjectReuseProjectedTotalsKeepDerivedStatus(t *testing.T) {
	t.Parallel()
	observedBytes := observed(100, "bytes")
	observedTokens := observed(50, "tokens")
	observedDuration := observed(20, "milliseconds")
	report := Report{
		Baseline: OperationCost{WallDurationMS: 10, BytesScanned: 1000},
		Update:   OperationCost{WallDurationMS: 5, BytesScanned: 200},
		Cue:      CueCost{WallDurationMS: 2, OutputBytes: 80, EstimatedTokens: 20},
		Direct: ArmResult{
			TotalTokens:         observedTokens,
			RepositoryBytesRead: observedBytes,
			ExecutionDuration:   observedDuration,
		},
		Assisted: ArmResult{
			TotalTokens:                 observedTokens,
			RepositoryBytesRead:         observedBytes,
			FallbackRepositoryBytesRead: observedBytes,
			ExecutionDuration:           observedDuration,
		},
	}

	projections := projectReuse(report)
	if len(projections) != len(consumerCounts) {
		t.Fatalf("got %d projections, want %d", len(projections), len(consumerCounts))
	}
	for index, projection := range projections {
		if projection.Consumers != consumerCounts[index] {
			t.Fatalf("projection %d consumers = %d, want %d", index, projection.Consumers, consumerCounts[index])
		}
		measurements := map[string]Measurement{
			"direct_total_tokens":               projection.DirectTotalTokens,
			"assisted_total_tokens":             projection.AssistedTotalTokens,
			"direct_repository_bytes_read":      projection.DirectRepositoryBytesRead,
			"assisted_repository_bytes_read":    projection.AssistedRepositoryBytesRead,
			"direct_execution_duration":         projection.DirectExecutionDuration,
			"assisted_total_execution_duration": projection.AssistedTotalExecutionDuration,
		}
		for name, measurement := range measurements {
			if measurement.Status != measurementDerived {
				t.Errorf("%d consumers: %s status = %q, want %q", projection.Consumers, name, measurement.Status, measurementDerived)
			}
		}
	}
}

func TestObservedArmScoresStructuredBenchmarkAnswer(t *testing.T) {
	branch := "main"
	head := "abc123"
	answer := benchmark.Answer{
		SchemaVersion:      benchmark.AnswerSchemaVersion,
		Git:                benchmark.GitState{Branch: &branch, Head: &head, TrackedChanges: []string{}, Untracked: []string{}},
		PrimaryEntryPoints: []benchmark.EntryPoint{}, MajorComponents: []benchmark.Component{},
		ImportantDocumentation: []benchmark.Document{}, RecentRelevantChanges: []string{}, Uncertainties: []string{},
	}
	serialized, err := json.Marshal(answer)
	if err != nil {
		t.Fatal(err)
	}
	result, err := observedArm(RunnerObservation{
		SchemaVersion: RunnerSchemaVersion, Arm: "direct",
		Metadata: RunnerMetadata{Adapter: "fixture", AdapterVersion: "1", OutputSchemaVersion: benchmark.AnswerSchemaVersion},
		Metrics:  RunnerMetrics{}, FinalResponse: serialized,
	}, 0, model.Basis{Branch: &branch, Head: &head, Staged: []string{}, Unstaged: []string{}, Untracked: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.DeterministicScore == nil || result.DeterministicScore.Ratio != 1 {
		t.Fatalf("unexpected deterministic score: %#v", result.DeterministicScore)
	}
	if result.ContextCorrectness.Status != measurementDerived {
		t.Fatalf("unexpected correctness source: %#v", result.ContextCorrectness)
	}
}

func createRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.name", "RepoCue Test")
	runGit(t, root, "config", "user.email", "repocue@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md", "cmd/main.go")
	runGit(t, root, "commit", "-m", "Initialize fixture")
	return root
}

func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, output)
	}
	return string(output)
}

func gitStatus(t *testing.T, root string) string {
	t.Helper()
	return runGit(t, root, "status", "--porcelain=v1", "--untracked-files=all")
}

func validateSchema(t *testing.T, name string, value any) {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "schema", name)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var schemaDocument any
	if err := json.NewDecoder(file).Decode(&schemaDocument); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(name, schemaDocument); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(name)
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(serialized, &document); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("%s validation failed: %v\n%s", name, err, serialized)
	}
}
