package evaluation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestConditionSetDryRunUsesRealOracleAndWritesAtomicReports(t *testing.T) {
	root := createM2Repository(t)
	reports := t.TempDir()
	manifest, err := RunM2(context.Background(), M2Config{
		RepositoryPath: root, MaxTokens: 500, OracleTool: realOraclePath(t),
		OutputDirectory: reports, RunIndex: 1, TemporaryRoot: t.TempDir(),
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
	if len(entries) != len(M2Conditions) {
		t.Fatalf("got %d reports, want %d", len(entries), len(M2Conditions))
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
	for _, entry := range entries {
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
printf '{"schema_version":"repocue/evaluation-runner-3","condition":"%s","run_index":%s,"metadata":{"adapter":"fixture","adapter_version":"1","model":"fixture","benchmark_version":"repository-state-v2","output_schema_version":"repocue/benchmark-answer-2"},"metrics":{},"usage_events":[],"tokenizer_counts":[],"commands":[],"findings":[],"limitations":[]}\n' "$REPOCUE_EVAL_CONDITION" "$REPOCUE_EVAL_RUN_INDEX"
`
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
	path := filepath.Join("..", "..", "docs", "schema", name)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var document any
	if err := json.NewDecoder(file).Decode(&document); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(name, document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(name)
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
