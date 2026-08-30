package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jeonghanlee/repocue/internal/app"
)

func TestVerticalSliceWithRealGitRepository(t *testing.T) {
	repository := createGitRepository(t)
	cache := t.TempDir()

	initialized := runCLI(t, "init", "--cache-dir", cache, repository)
	if initialized["snapshot"] != "snapshot-000001" {
		t.Fatalf("unexpected initial snapshot: %#v", initialized["snapshot"])
	}

	status := runCLI(t, "status", "--cache-dir", cache, "--repository", repository)
	if status["freshness"] != "current" {
		t.Fatalf("unexpected initial freshness: %#v", status["freshness"])
	}

	overview := runCLI(t, "cue", "--cache-dir", cache, "--repository", repository, "--view", "overview", "--max-tokens", "500")
	budget := overview["budget"].(map[string]any)
	if budget["estimated_tokens"].(float64) > 500 {
		t.Fatalf("overview exceeded token budget: %#v", budget)
	}

	writeFile(t, filepath.Join(repository, "README.md"), "# Example\n\nChanged.\n")
	writeFile(t, filepath.Join(repository, "docs", "new.md"), "# New\n")
	runGit(t, repository, "add", "docs/new.md")

	refresh := runCLI(t, "refresh", "--cache-dir", cache, "--repository", repository)
	if refresh["snapshot"] != "snapshot-000002" || refresh["changed"] != true {
		t.Fatalf("unexpected refresh output: %#v", refresh)
	}
	refreshMetrics := refresh["metrics"].(map[string]any)
	if refreshMetrics["files_scanned"].(float64) != 2 {
		t.Fatalf("incremental refresh scanned unexpected file count: %#v", refreshMetrics)
	}

	delta := runCLI(t, "cue", "--cache-dir", cache, "--repository", repository, "--since", "snapshot-000001", "--max-tokens", "1000")
	content := delta["content"].(map[string]any)
	if content["change_count"].(float64) < 2 {
		t.Fatalf("semantic delta omitted changes: %#v", content)
	}
	noOp := runCLI(t, "refresh", "--cache-dir", cache, "--repository", repository)
	if noOp["snapshot"] != "snapshot-000002" || noOp["changed"] != false {
		t.Fatalf("no-op refresh created state: %#v", noOp)
	}

	rebaseline := runCLI(t, "rebaseline", "--cache-dir", cache, "--repository", repository, "--label", "milestone:first-prototype")
	if rebaseline["epoch"] != "epoch-000002" || rebaseline["superseded_epoch"] != "epoch-000001" {
		t.Fatalf("unexpected rebaseline output: %#v", rebaseline)
	}

	status = runCLI(t, "status", "--cache-dir", cache, "--repository", repository)
	if status["epoch_count"].(float64) != 2 {
		t.Fatalf("superseded epoch was not retained: %#v", status["epoch_count"])
	}

	metrics := runCLI(t, "metrics", "--cache-dir", cache, "--repository", repository)
	metricContent := metrics["content"].(map[string]any)
	if metricContent["database_size_bytes"].(float64) <= 0 {
		t.Fatalf("database size was not measured: %#v", metricContent)
	}
	operations := metricContent["operations"].([]any)
	if len(operations) != 4 {
		t.Fatalf("got %d operation metrics, want 4", len(operations))
	}
	if len(metricContent["cues"].([]any)) != 2 {
		t.Fatalf("cue metrics were not recorded: %#v", metricContent["cues"])
	}
}

func TestExperimentalCueViewsAndM2DryRun(t *testing.T) {
	repository := createGitRepository(t)
	cache := t.TempDir()
	runCLI(t, "init", "--cache-dir", cache, repository)
	for _, view := range []string{"placebo", "ranked", "provenance"} {
		result := runCLI(t, "cue", "--cache-dir", cache, "--repository", repository, "--view", view, "--max-tokens", "500")
		if result["schema_version"] != "repocue/cue-2" || result["kind"] != view {
			t.Fatalf("unexpected %s cue: %#v", view, result)
		}
		budget := result["budget"].(map[string]any)
		if budget["estimated_tokens"].(float64) > 500 {
			t.Fatalf("%s cue exceeded budget: %#v", view, budget)
		}
	}
	oracle, err := filepath.Abs(filepath.Join("..", "..", "tools", "evaluation", "structural-oracle.bash"))
	if err != nil {
		t.Fatal(err)
	}
	reports := t.TempDir()
	result := runCLI(t, "evaluate-m2", "--repository", repository, "--oracle-tool", oracle,
		"--output-directory", reports, "--max-tokens", "500", "--run-index", "1")
	if result["kind"] != "m2-condition-set" {
		t.Fatalf("unexpected M2 manifest: %#v", result)
	}
	if got := len(result["reports"].([]any)); got != 5 {
		t.Fatalf("got %d M2 reports, want 5", got)
	}
	entries, err := os.ReadDir(reports)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("got %d final report files, want 5", len(entries))
	}
}

func createGitRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	runGit(t, repository, "init", "-b", "main")
	runGit(t, repository, "config", "user.name", "RepoCue Test")
	runGit(t, repository, "config", "user.email", "repocue@example.invalid")
	writeFile(t, filepath.Join(repository, "README.md"), "# Example\n")
	writeFile(t, filepath.Join(repository, "cmd", "example", "main.go"), "package main\n")
	runGit(t, repository, "add", "README.md", "cmd/example/main.go")
	runGit(t, repository, "commit", "-m", "Initialize fixture")
	return repository
}

func runCLI(t *testing.T, args ...string) map[string]any {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := app.Run(context.Background(), args, &stdout, &stderr); code != 0 {
		t.Fatalf("repocue %v failed with code %d: %s", args, code, stderr.String())
	}
	var output map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode output %q: %v", stdout.String(), err)
	}
	return output
}

func runGit(t *testing.T, repository string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", repository}, args...)
	command := exec.Command("git", commandArgs...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, output)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
