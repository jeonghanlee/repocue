package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/docs/schema"
	"github.com/jeonghanlee/repocue/internal/benchmark"
	"github.com/jeonghanlee/repocue/internal/codexadapter"
	"github.com/jeonghanlee/repocue/internal/evaluation"
	"github.com/jeonghanlee/repocue/internal/evidence"
	"github.com/jeonghanlee/repocue/internal/reporting"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/storage"
)

// TestReportingRunnerBoundary is the external agent replacement only. The
// evaluator, shipped cue executable, importer, SQLite, and report remain real.
func TestReportingRunnerBoundary(t *testing.T) {
	if os.Getenv("REPOCUE_TEST_AGENT_BOUNDARY") != "1" {
		return
	}
	condition := os.Getenv("REPOCUE_EVAL_CONDITION")
	answer, err := os.ReadFile(os.Getenv("REPOCUE_TEST_ANSWER_FILE"))
	if err != nil {
		os.Exit(41)
	}
	if path := os.Getenv("REPOCUE_EVAL_CUE_FILE"); path != "" {
		cue, err := os.ReadFile(path)
		if err != nil {
			os.Exit(42)
		}
		if err := os.WriteFile(os.Getenv("REPOCUE_TEST_DELIVERED_DIGEST"), []byte(evidence.Digest(cue)), 0o600); err != nil {
			os.Exit(43)
		}
	}
	if path := os.Getenv("REPOCUE_TEST_MUTATE_FILE"); path != "" {
		if err := os.WriteFile(path, []byte("changed by external fixture"), 0o600); err != nil {
			os.Exit(44)
		}
	}
	input, output, zero := int64(1000), int64(10), int64(0)
	if condition == evaluation.ConditionBasic {
		input = 100
	}
	total := input + output
	duration := 10.0
	runIndex := 1
	if _, err := fmt.Sscan(os.Getenv("REPOCUE_EVAL_RUN_INDEX"), &runIndex); err != nil {
		os.Exit(45)
	}
	observation := evaluation.M2RunnerObservation{
		SchemaVersion: evaluation.M2RunnerSchemaVersion, Condition: condition, RunIndex: runIndex,
		Metadata: evaluation.RunnerMetadata{Adapter: "external-fixture", AdapterVersion: "1", Model: "synthetic-fixture", ReasoningEffort: "medium", Sandbox: "read-only",
			BenchmarkVersion: benchmark.Version, OutputSchemaVersion: benchmark.AnswerSchemaVersion, RepositoryHead: os.Getenv("REPOCUE_EVAL_HEAD"), RepoCueSnapshot: os.Getenv("REPOCUE_EVAL_SNAPSHOT")},
		Metrics: evaluation.M2RunnerMetrics{InputTokens: &input, CachedInputTokens: &zero, OutputTokens: &output, ReasoningOutputTokens: &zero, TotalTokens: &total,
			ExecutionDurationMS: &duration, CommandExecutions: &zero, GitCalls: &zero, RepositoryFilesNamed: &zero, Statuses: map[string]string{
				"input_tokens": "observed", "cached_input_tokens": "observed", "output_tokens": "observed", "reasoning_output_tokens": "observed", "total_tokens": "derived",
				"execution_duration_ms": "observed", "command_executions": "observed", "git_calls": "derived", "repository_files_named": "derived"}},
		UsageEvents: []evaluation.UsageEvent{{Turn: 1, InputTokens: &input, CachedInputTokens: &zero, OutputTokens: &output, ReasoningOutputTokens: &zero}}, TokenizerCounts: []evaluation.TokenizerCount{}, Commands: []evaluation.CommandObservation{},
		FinalResponse: answer, Findings: []evaluation.ContextFinding{}, Limitations: []string{"synthetic external fixture, not real agent measurements"},
	}
	if err := json.NewEncoder(os.Stdout).Encode(observation); err != nil {
		os.Exit(46)
	}
	os.Exit(0)
}

func TestReportingEstimatedAgentCostsStayDiagnostic(t *testing.T) {
	f := prepareReportingFixture(t)
	for _, condition := range []string{evaluation.ConditionDirect, evaluation.ConditionBasic, "both"} {
		t.Run(condition, func(t *testing.T) {
			reports, proof := fixtureInputs(t, f, func(_ *evidence.Document, reports map[string]*evaluation.M2ConditionReport) {
				for name, report := range reports {
					if condition != "both" && condition != name {
						continue
					}
					for _, metric := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens", "execution_duration_ms"} {
						report.Runner.Metrics.Statuses[metric] = "estimated"
					}
				}
			})
			id := fixtureImport(t, f, reports, proof)
			report := fixtureReport(t, f, "--profile", id)
			pair := report.Calibration.Pairs[0]
			for _, sample := range []reporting.Sample{pair.Direct, pair.Basic} {
				if condition == "both" || condition == sample.Condition {
					value := sample.Metrics["total_tokens"]
					if value.Status != "estimated" || value.Value == nil {
						t.Fatal("source estimate was discarded or relabeled")
					}
				}
			}
			for _, metric := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens", "execution_duration_ms"} {
				comparison := report.Calibration.Comparisons[metric]
				if comparison.Difference.Status != "not_observed" || comparison.Difference.Count != 0 || comparison.Difference.Unknown != 1 || comparison.Difference.Median != nil {
					t.Fatalf("estimated %s entered paired statistics", metric)
				}
				for _, projection := range report.Projections {
					if projection.Metric == metric && (projection.Saving.Median != nil || projection.FiniteBreakEven.Median != nil || projection.Saving.Unknown != 1) {
						t.Fatalf("estimated %s entered consumer projections", metric)
					}
				}
			}
			commands := report.Calibration.Comparisons["command_executions"].Difference
			if commands.Count != 1 || commands.Median == nil || *commands.Median != 0 {
				t.Fatal("excluded estimate erased an unrelated observed zero")
			}
		})
	}
}

// TestReportingCodexBoundary replaces only the external Codex process. Its
// JSONL passes through the shipped adapter, evaluator, importer, and report.
func TestReportingCodexBoundary(t *testing.T) {
	if os.Getenv("REPOCUE_TEST_CODEX_BOUNDARY") != "1" {
		return
	}
	answer, err := os.ReadFile(os.Getenv("REPOCUE_TEST_ANSWER_FILE"))
	if err != nil {
		os.Exit(51)
	}
	if _, err := os.ReadFile("README.md"); err != nil {
		os.Exit(52)
	}
	encoder := json.NewEncoder(os.Stdout)
	for _, event := range []any{
		map[string]any{"type": "item.completed", "item": map[string]string{"type": "command_execution", "command": "cat README.md"}},
		map[string]any{"type": "item.completed", "item": map[string]string{"type": "agent_message", "text": string(answer)}},
	} {
		if err := encoder.Encode(event); err != nil {
			os.Exit(53)
		}
	}
	input := "1000"
	if os.Getenv("REPOCUE_EVAL_CONDITION") == evaluation.ConditionBasic {
		input = "100"
	}
	usage := strings.ReplaceAll(os.Getenv("REPOCUE_TEST_CODEX_USAGE"), "INPUT", input)
	if _, err := fmt.Fprintln(os.Stdout, usage); err != nil {
		os.Exit(54)
	}
	os.Exit(0)
}

func TestReportingCodexUsageThroughShippedPipeline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the external Codex fixture is a POSIX executable")
	}
	f := prepareReportingFixture(t)
	adapter := filepath.Join(f.external, "codex-runner")
	build := exec.Command("go", "build", "-o", adapter, "./cmd/repocue-codex-runner")
	build.Dir = filepath.Join("..", "..")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build adapter: %v: %s", err, output)
	}
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	codex := filepath.Join(f.external, "codex")
	quoted := "'" + strings.ReplaceAll(testExecutable, "'", "'\\''") + "'"
	writeFile(t, codex, "#!/bin/sh\nset -eu\nif test \"$1\" = \"--version\"; then printf '%s\\n' 'codex-cli fixture'; exit 0; fi\ntest \"$1\" = \"exec\"\ncat >/dev/null\nREPOCUE_TEST_CODEX_BOUNDARY=1\nexport REPOCUE_TEST_CODEX_BOUNDARY\nexec "+quoted+" -test.run=^TestReportingCodexBoundary$\n")
	if err := os.Chmod(codex, 0o700); err != nil {
		t.Fatal(err)
	}
	answerSchema, err := filepath.Abs(filepath.Join("..", "..", "docs", "schema", "benchmark-answer-v2.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("REPOCUE_CODEX_BIN", codex)
	t.Setenv("REPOCUE_CODEX_MODEL", "synthetic-codex-fixture")
	t.Setenv("REPOCUE_CODEX_REASONING_EFFORT", "medium")
	t.Setenv("REPOCUE_CODEX_OUTPUT_SCHEMA_FILE", answerSchema)
	repo, err := repository.Open(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.CaptureEvidence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for index, test := range []struct {
		name      string
		usage     string
		unknown   []string
		zeroInput bool
	}{
		{"omitted reasoning", `{"type":"turn.completed","usage":{"input_tokens":INPUT,"cached_input_tokens":0,"output_tokens":10}}`, []string{"reasoning_output_tokens"}, false},
		{"null subsets", `{"type":"turn.completed","usage":{"input_tokens":INPUT,"cached_input_tokens":null,"output_tokens":10,"reasoning_output_tokens":null}}`, []string{"cached_input_tokens", "reasoning_output_tokens"}, false},
		{"partial input", "{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":INPUT,\"cached_input_tokens\":0,\"output_tokens\":10,\"reasoning_output_tokens\":0}}\n{\"type\":\"turn.completed\",\"usage\":{\"cached_input_tokens\":0,\"output_tokens\":2,\"reasoning_output_tokens\":0}}", []string{"input_tokens", "total_tokens"}, false},
		{"absent usage", `{"type":"turn.completed"}`, []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens"}, false},
		{"explicit zero", `{"type":"turn.completed","usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0}}`, nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("REPOCUE_TEST_CODEX_USAGE", test.usage)
			out := filepath.Join(t.TempDir(), "sets")
			data := fixtureCLI(t, f, "evaluate-m2", "--repository", f.root, "--conditions", "direct,basic", "--max-tokens", "500", "--runner", adapter,
				"--task-file", f.answer, "--run-index", fmt.Sprint(index+1), "--output-directory", out)
			var manifest evaluation.M2Manifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.SchemaVersion != evaluation.M2SchemaVersion || len(manifest.Reports) != 2 {
				t.Fatal("invalid current report set")
			}
			for _, report := range manifest.Reports {
				if report.Runner.SchemaVersion != evaluation.M2RunnerSchemaVersion || report.Runner.Metadata.AdapterVersion != codexadapter.AdapterVersion || report.Runner.Metadata.Adapter != "codex-cli" {
					t.Fatal("shipped Codex adapter was not used")
				}
			}
			sets, err := os.ReadDir(out)
			if err != nil || len(sets) != 1 {
				t.Fatalf("published sets: %v %v", sets, err)
			}
			reports := filepath.Join(out, sets[0].Name())
			id := fixtureImport(t, f, reports, filepath.Join(reports, "calibration-evidence.json"))
			result := fixtureReport(t, f, "--profile", id)
			if result.Applicability.Status != "measured_reference" || len(result.Calibration.Pairs) != 1 {
				t.Fatal("current profile is not applicable")
			}
			for _, sample := range []reporting.Sample{result.Calibration.Pairs[0].Direct, result.Calibration.Pairs[0].Basic} {
				if sample.Correctness.Ratio == nil || *sample.Correctness.Ratio != 1 {
					t.Fatal("usage absence erased deterministic correctness")
				}
				for _, name := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens", "command_executions", "execution_duration_ms"} {
					metric := sample.Metrics[name]
					if slices.Contains(test.unknown, name) {
						if metric.Value != nil || metric.Status != "not_observed" {
							t.Fatalf("%s became observed: %+v", name, metric)
						}
					} else if metric.Value == nil || metric.Status == "not_observed" {
						t.Fatalf("%s was erased", name)
					}
				}
			}
			for name, comparison := range result.Calibration.Comparisons {
				if name == "execution_duration_ms" || !strings.Contains(name, "tokens") {
					continue
				}
				if slices.Contains(test.unknown, name) {
					if comparison.Difference.Count != 0 || comparison.Difference.Unknown != 1 || comparison.Difference.Median != nil {
						t.Fatalf("%s partial usage entered paired statistics", name)
					}
				} else {
					want := 0.0
					if !test.zeroInput && (name == "input_tokens" || name == "total_tokens") {
						want = 900
					}
					if comparison.Difference.Count != 1 || comparison.Difference.Median == nil || *comparison.Difference.Median != want {
						t.Fatalf("%s complete usage changed: %+v", name, comparison)
					}
				}
			}
			for _, projection := range result.Projections {
				if slices.Contains(test.unknown, projection.Metric) && (projection.Saving.Median != nil || projection.Saving.Unknown != 1) {
					t.Fatalf("%s unknown usage entered projections", projection.Metric)
				}
			}
			markdown := fixtureCLI(t, f, "report", "--repository", f.root, "--cache-dir", f.cache, "--profile", id, "--format", "markdown")
			_, block, found := strings.Cut(string(markdown), "```json\n")
			if !found {
				t.Fatal("Markdown report has no complete measurements")
			}
			block, _, found = strings.Cut(block, "\n```")
			if !found {
				t.Fatal("Markdown report is incomplete")
			}
			var markdownReport reporting.Report
			if err := json.Unmarshal([]byte(block), &markdownReport); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Calibration.Pairs, markdownReport.Calibration.Pairs) {
				t.Fatal("JSON and Markdown token measurements differ")
			}
		})
	}
	after, err := repo.CaptureEvidence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if before.Evidence.Fingerprint != after.Evidence.Fingerprint {
		t.Fatal("pipeline changed the evaluated repository")
	}
}

func TestReportingCueObservationMustPrecedeAssistedRun(t *testing.T) {
	f := prepareReportingFixture(t)
	for _, placement := range []string{"after-run", "overlapping-pre-run-capture"} {
		t.Run(placement, func(t *testing.T) {
			reports, proof := fixtureInputs(t, f, func(proof *evidence.Document, _ map[string]*evaluation.M2ConditionReport) {
				cli := &proof.CueCLI[0]
				if placement == "after-run" {
					cli.StartedAt = cli.StartedAt.Add(24 * time.Hour)
					cli.CompletedAt = cli.CompletedAt.Add(24 * time.Hour)
				} else {
					for _, ref := range proof.Reports {
						if ref.Condition == evaluation.ConditionBasic {
							cli.CompletedAt = ref.Before.ObservedAt.Add(time.Nanosecond)
						}
					}
					cli.DurationMS = float64(cli.CompletedAt.Sub(cli.StartedAt)) / float64(time.Millisecond)
				}
			})
			fixtureFailure(t, f, "calibration", "import", "--repository", f.root, "--cache-dir", f.cache, "--reports", reports, "--evidence", proof)
			if report := fixtureReport(t, f); len(report.Profiles) != 0 {
				t.Fatal("contradictory cue chronology published a reference")
			}
		})
	}
	id := fixtureImport(t, f, f.reports, f.proof)
	if report := fixtureReport(t, f, "--profile", id); report.Applicability.Status != "measured_reference" {
		t.Fatal("real producer chronology was rejected")
	}
}

func TestReportingImportsThreeRealPreparedPairs(t *testing.T) {
	f := prepareReportingFixture(t)
	sources := []string{f.reports}
	for _, index := range []string{"2", "3"} {
		parent := filepath.Join(f.external, "run-"+index)
		fixtureCLI(t, f, "evaluate-m2", "--repository", f.root, "--conditions", "direct,basic", "--runner", filepath.Join(f.external, "runner"), "--task-file", f.answer, "--output-directory", parent, "--run-index", index)
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) != 1 {
			t.Fatalf("pair output: %v %v", entries, err)
		}
		sources = append(sources, filepath.Join(parent, entries[0].Name()))
	}
	merged := t.TempDir()
	var combined evidence.Document
	for index, source := range sources {
		prefix := fmt.Sprintf("pair-%d", index+1)
		if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			writeFile(t, filepath.Join(merged, prefix, relative), string(data))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(source, "calibration-evidence.json"))
		if err != nil {
			t.Fatal(err)
		}
		var proof evidence.Document
		if err := json.Unmarshal(data, &proof); err != nil {
			t.Fatal(err)
		}
		for index := range proof.Reports {
			proof.Reports[index].Path = prefix + "/" + proof.Reports[index].Path
		}
		for index := range proof.Records {
			proof.Records[index].Path = prefix + "/" + proof.Records[index].Path
		}
		if index == 0 {
			combined = proof
			continue
		}
		combined.Reports = append(combined.Reports, proof.Reports...)
		combined.Scoring = append(combined.Scoring, proof.Scoring...)
		combined.Records = append(combined.Records, proof.Records...)
		combined.CueCLI = append(combined.CueCLI, proof.CueCLI...)
	}
	data, err := json.Marshal(combined)
	if err != nil {
		t.Fatal(err)
	}
	proof := filepath.Join(merged, "calibration-evidence.json")
	writeFile(t, proof, string(data))
	id := fixtureImport(t, f, merged, proof)
	report := fixtureReport(t, f, "--profile", id)
	if report.Calibration.SampleGrade != "descriptive" || len(report.Calibration.Pairs) != 3 {
		t.Fatal("complete pairs were lost")
	}
	if comparison := report.Calibration.Comparisons["total_tokens"]; comparison.PositivePairs != 3 || comparison.Difference.Count != 3 || *comparison.Difference.Median != 900 {
		t.Fatalf("paired fixture costs: %+v", comparison)
	}
	for _, projection := range report.Projections {
		if projection.Metric == "total_tokens" && projection.Saving.Count != 3 {
			t.Fatal("projection lost pair association")
		}
	}
}

func TestReportingMissingFullCueObservationDoesNotUseBuilderCost(t *testing.T) {
	f := prepareReportingFixture(t)
	reports, proof := fixtureInputs(t, f, func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) { p.CueCLI = []evidence.CueCLI{} })
	id := fixtureImport(t, f, reports, proof)
	report := fixtureReport(t, f, "--profile", id)
	if report.Calibration.Pairs[0].Cue["execution_duration_ms"].Value != nil || report.Calibration.Pairs[0].Cue["builder_duration_ms"].Value == nil {
		t.Fatal("builder and full CLI cost were conflated")
	}
	for _, projection := range report.Projections {
		if projection.Metric == "execution_duration_ms" && (projection.Saving.Median != nil || projection.FiniteBreakEven.Median != nil) {
			t.Fatal("missing full cue cost was filled")
		}
		if projection.Metric == "total_tokens" && projection.Saving.Median == nil {
			t.Fatal("missing duration erased observed token pairs")
		}
	}
}

func TestReportingImportRejectsAnotherCheckoutAndChangedIndex(t *testing.T) {
	f := prepareReportingFixture(t)
	other := f
	other.root = createGitRepository(t)
	other.cache = t.TempDir()
	fixtureCLI(t, other, "init", "--cache-dir", other.cache, other.root)
	fixtureFailure(t, other, "calibration", "import", "--repository", other.root, "--cache-dir", other.cache, "--reports", f.reports, "--evidence", f.proof)
	writeFile(t, filepath.Join(f.root, "README.md"), "staged change\n")
	runGit(t, f.root, "add", "README.md")
	fixtureCLI(t, f, "refresh", "--repository", f.root, "--cache-dir", f.cache)
	fixtureFailure(t, f, "calibration", "import", "--repository", f.root, "--cache-dir", f.cache, "--reports", f.reports, "--evidence", f.proof)
	if len(fixtureReport(t, f).Profiles) != 0 || len(fixtureReport(t, other).Profiles) != 0 {
		t.Fatal("mismatched basis published calibration")
	}
}

func TestReportingDocumentedPublisherPreservesLastSuccess(t *testing.T) {
	f := prepareReportingFixture(t)
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "CLI.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(doc), "### Atomic Publication Wrapper")
	if !ok {
		t.Fatal("publication section absent")
	}
	_, block, ok := strings.Cut(section, "```bash\n")
	if !ok {
		t.Fatal("publication script absent")
	}
	script, _, ok := strings.Cut(block, "\n```")
	if !ok {
		t.Fatal("publication script incomplete")
	}
	destination := filepath.Join(f.external, "published")
	for original, value := range map[string]string{"/home/example/.local/bin/repocue": f.binary, "/srv/project": f.root, "/home/example/.cache/repocue": f.cache, "/home/example/reports/repocue": destination} {
		script = strings.ReplaceAll(script, original, "'"+strings.ReplaceAll(value, "'", "'\\''")+"'")
	}
	path := filepath.Join(f.external, "publish.bash")
	writeFile(t, path, script+"\n")
	if output, err := exec.Command("bash", path).CombinedOutput(); err != nil {
		t.Fatalf("documented publisher: %v %s", err, output)
	}
	published := filepath.Join(destination, "current.md")
	before, err := os.ReadFile(published)
	if err != nil || len(before) == 0 {
		t.Fatalf("no published report: %v", err)
	}
	repo, err := repository.Open(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(f.cache, repo.ID, "state.db")
	if err := os.Rename(db, db+".saved"); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("bash", path).CombinedOutput(); err == nil || !bytes.Contains(output, []byte("Failed draft:")) {
		t.Fatalf("documented failure path: %v %s", err, output)
	}
	after, err := os.ReadFile(published)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed publisher overwrote last success")
	}
}

func TestReportingCLIConcurrentReadersRefreshAndImport(t *testing.T) {
	f := prepareReportingFixture(t)
	id := fixtureImport(t, f, f.reports, f.proof)
	var workers sync.WaitGroup
	results := make(chan error, 12)
	for index := 0; index < 12; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			args := []string{"report", "--repository", f.root, "--cache-dir", f.cache, "--profile", id}
			if index%3 == 1 {
				args = []string{"refresh", "--repository", f.root, "--cache-dir", f.cache}
			}
			if index%3 == 2 {
				args = []string{"calibration", "import", "--repository", f.root, "--cache-dir", f.cache, "--reports", f.reports, "--evidence", f.proof}
			}
			command := exec.Command(f.binary, args...)
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			if err := command.Run(); err != nil {
				if strings.Contains(stderr.String(), "locked") || strings.Contains(stderr.String(), "concurrent") {
					return
				}
				results <- fmt.Errorf("concurrent CLI: %v %s", err, stderr.String())
				return
			}
			if index%3 == 0 {
				if err := schema.Validate("savings-report-v1.schema.json", stdout.Bytes()); err != nil {
					results <- err
					return
				}
				var report reporting.Report
				if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
					results <- err
					return
				}
				if report.State == nil || report.Calibration == nil || report.State.Snapshot != report.Calibration.Binding.Snapshot || report.Applicability.Status != "measured_reference" {
					results <- fmt.Errorf("partial concurrent report")
				}
			}
		}(index)
	}
	workers.Wait()
	close(results)
	for err := range results {
		t.Error(err)
	}
	if report := fixtureReport(t, f); len(report.Profiles) != 1 || report.Applicability.Status != "measured_reference" {
		t.Fatal("concurrent operation corrupted final state")
	}
}

func TestReportingChangedSnapshotAndProducerContract(t *testing.T) {
	f := prepareReportingFixture(t)
	id := fixtureImport(t, f, f.reports, f.proof)
	before := fixtureReport(t, f, "--profile", id)
	other := f
	other.binary = filepath.Join(t.TempDir(), "repocue-other-build")
	build := exec.Command("go", "build", "-ldflags=-s -w", "-o", other.binary, "./cmd/repocue")
	build.Dir = filepath.Join("..", "..")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("alternate shipped build: %v %s", err, output)
	}
	if changed := fixtureReport(t, other, "--profile", id); changed.Applicability.Status != "not_applicable" || len(changed.Projections) != 0 {
		t.Fatal("incompatible executable reused calibration")
	}
	writeFile(t, filepath.Join(f.root, "README.md"), "changed snapshot for concurrent reporting\n")
	var workers sync.WaitGroup
	results := make(chan error, 8)
	for index := 0; index < 8; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			args := []string{"report", "--repository", f.root, "--cache-dir", f.cache, "--profile", id}
			if index == 0 {
				args = []string{"refresh", "--repository", f.root, "--cache-dir", f.cache}
			}
			command := exec.Command(f.binary, args...)
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			if err := command.Run(); err != nil {
				results <- fmt.Errorf("snapshot contention: %v %s", err, stderr.String())
				return
			}
			if index == 0 {
				return
			}
			if err := schema.Validate("savings-report-v1.schema.json", stdout.Bytes()); err != nil {
				results <- err
				return
			}
			var report reporting.Report
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				results <- err
				return
			}
			if report.State.Snapshot == before.State.Snapshot {
				// State selection uses the command-start cutoff; freshness describes
				// the current cache observed by the later read transaction.
				if report.Applicability.Status != "not_applicable" || len(report.Projections) != 0 || !slices.Contains(report.Applicability.Reasons, "refresh-needed") {
					results <- fmt.Errorf("old selected state retained live projections")
				}
			} else if report.Applicability.Status != "estimated" || report.Applicability.TrackedByteDrift == 0 {
				results <- fmt.Errorf("new state lacks compatible drift evidence")
			}
		}(index)
	}
	workers.Wait()
	close(results)
	for err := range results {
		t.Error(err)
	}
	after := fixtureReport(t, f, "--profile", id)
	if after.State.Snapshot == before.State.Snapshot || after.Applicability.Status != "estimated" {
		t.Fatal("real changed refresh did not publish a coherent snapshot")
	}
}

func TestReportingLegacyDBCoverageBeforeAndAfterImport(t *testing.T) {
	f := prepareReportingFixture(t)
	repo, err := repository.Open(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.cache, repo.ID, "state.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE calibration_references; ALTER TABLE operation_runs DROP COLUMN scan_duration_ms; ALTER TABLE operation_runs DROP COLUMN git_commands; PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	fixtureCLI(t, f, "refresh", "--repository", f.root, "--cache-dir", f.cache)
	before := fixtureReport(t, f)
	if before.DatabaseSchema != 3 || before.Maintenance.Total["git_invocations"].Value != nil || before.Maintenance.Total["scan_duration_ms"].Value != nil || before.Maintenance.Total["execution_duration_ms"].Value == nil {
		t.Fatal("legacy metric coverage is wrong")
	}
	id := fixtureImport(t, f, f.reports, f.proof)
	after := fixtureReport(t, f, "--profile", id)
	if !reflect.DeepEqual(before.Maintenance, after.Maintenance) {
		// Only the command's observation cutoff changes; history values do not.
		before.Maintenance.Cutoff = after.Maintenance.Cutoff
		if !reflect.DeepEqual(before.Maintenance, after.Maintenance) {
			t.Fatal("import changed legacy measurement interpretation")
		}
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var scan float64
	var git int
	if err := db.QueryRow("SELECT scan_duration_ms, git_commands FROM operation_runs LIMIT 1").Scan(&scan, &git); err != nil || scan != 0 || git != 0 {
		t.Fatalf("raw legacy zeros rewritten: %g %d %v", scan, git, err)
	}
}

func TestReportingVersionTwoReadOnlyAndPrivateCalibrationStorage(t *testing.T) {
	f := prepareReportingFixture(t)
	repo, err := repository.Open(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.cache, repo.ID, "state.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE calibration_references; PRAGMA user_version=2"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	old := fixtureReport(t, f)
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || old.DatabaseSchema != 2 {
		t.Fatal("report migrated or changed version-2 DB")
	}
	id := fixtureImport(t, f, f.reports, f.proof)
	report := fixtureReport(t, f, "--profile", id)
	if report.DatabaseSchema != 3 {
		t.Fatal("import did not migrate version-2 DB")
	}
	for _, format := range []string{"json", "markdown"} {
		data := fixtureCLI(t, f, "report", "--repository", f.root, "--cache-dir", f.cache, "--format", format)
		for _, forbidden := range []string{"PRIVATE_FIXTURE_RESPONSE_TEXT", f.root, f.external} {
			if bytes.Contains(data, []byte(forbidden)) {
				t.Fatal("private text in report output")
			}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || bytes.Contains(data, []byte("PRIVATE_FIXTURE_RESPONSE_TEXT")) {
		t.Fatal("raw answer in SQLite")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var relative string
	if err := db.QueryRow("SELECT artifact_path FROM calibration_references WHERE id=?", id).Scan(&relative); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(filepath.Dir(path), relative))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"PRIVATE_FIXTURE_RESPONSE_TEXT", f.root, f.external} {
		if bytes.Contains(data, []byte(forbidden)) {
			t.Fatal("private text in calibration artifact")
		}
	}
	writeFile(t, filepath.Join(filepath.Dir(path), relative), "tampered")
	fixtureFailure(t, f, "report", "--repository", f.root, "--cache-dir", f.cache, "--profile", id)
}

func TestReportingCutoffBeforeMetricAndOutputFailure(t *testing.T) {
	f := prepareReportingFixture(t)
	initial := fixtureReport(t, f)
	cutoff := initial.State.ObservedAt.Add(time.Nanosecond).Format(time.RFC3339Nano)
	beforeMetric := fixtureReport(t, f, "--as-of", cutoff)
	if beforeMetric.State == nil || beforeMetric.Maintenance.Total["execution_duration_ms"].Value != nil {
		t.Fatal("state observation invented an earlier baseline recording")
	}
	id := fixtureImport(t, f, f.reports, f.proof)
	laterBinding := fixtureReport(t, f, "--as-of", cutoff, "--profile", id)
	if laterBinding.Calibration == nil || len(laterBinding.Projections) != 0 || laterBinding.Applicability.Status != "not_applicable" {
		t.Fatal("post-cutoff binding became applicable")
	}
	prior := filepath.Join(f.external, "published-report.json")
	writeFile(t, prior, "previous published report\n")
	full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	command := exec.Command(f.binary, "report", "--repository", f.root, "--cache-dir", f.cache)
	command.Stdout = full
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err == nil || stderr.Len() == 0 {
		t.Fatal("output failure was hidden")
	}
	data, err := os.ReadFile(prior)
	if err != nil || string(data) != "previous published report\n" {
		t.Fatal("failed caller output replaced previous publication")
	}
}

// fixtureInputs changes only versioned external inputs. Every import still
// executes the built application, real schemas, scorer, and SQLite transaction.
func fixtureInputs(t *testing.T, f reportingFixture, edit func(*evidence.Document, map[string]*evaluation.M2ConditionReport)) (string, string) {
	t.Helper()
	directory := t.TempDir()
	data, err := os.ReadFile(f.proof)
	if err != nil {
		t.Fatal(err)
	}
	var proof evidence.Document
	if err := json.Unmarshal(data, &proof); err != nil {
		t.Fatal(err)
	}
	reports := map[string]*evaluation.M2ConditionReport{}
	for _, ref := range proof.Reports {
		data, err := evidence.ReadReference(f.reports, ref.FileReference)
		if err != nil {
			t.Fatal(err)
		}
		var report evaluation.M2ConditionReport
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		reports[ref.Condition] = &report
	}
	for _, ref := range proof.Records {
		data, err := evidence.ReadReference(f.reports, ref)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(directory, ref.Path), string(data))
	}
	edit(&proof, reports)
	for index := range proof.Reports {
		ref := &proof.Reports[index]
		report := reports[ref.Condition]
		if report == nil {
			continue
		}
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, '\n')
		old := ref.Digest
		ref.Digest = evidence.Digest(data)
		for index := range proof.Scoring {
			if proof.Scoring[index].ReportDigest == old {
				proof.Scoring[index].ReportDigest = ref.Digest
			}
		}
		writeFile(t, filepath.Join(directory, ref.Path), string(data))
	}
	data, err = json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "calibration-evidence.json")
	writeFile(t, path, string(data))
	return directory, path
}

func fixtureImport(t *testing.T, f reportingFixture, reports, proof string, extra ...string) string {
	t.Helper()
	args := append([]string{"calibration", "import", "--repository", f.root, "--cache-dir", f.cache, "--reports", reports, "--evidence", proof}, extra...)
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(fixtureCLI(t, f, args...), &result); err != nil {
		t.Fatal(err)
	}
	return result.ID
}

func fixtureFailure(t *testing.T, f reportingFixture, args ...string) {
	t.Helper()
	command := exec.Command(f.binary, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err == nil || stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("expected explicit failure without stdout: %v %s %s", err, stdout.String(), stderr.String())
	}
}

func TestReportingImportRejectsIncompatibleExternalInputs(t *testing.T) {
	f := prepareReportingFixture(t)
	for _, test := range []struct {
		name string
		edit func(*evidence.Document, map[string]*evaluation.M2ConditionReport)
	}{
		{"incomplete", func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) { p.Reports = p.Reports[:1] }},
		{"cross repository", func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) {
			p.Capture.RepositoryID = "another-repository"
		}},
		{"pair index", func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) { p.Reports[0].RunIndex++ }},
		{"mixed model", func(_ *evidence.Document, r map[string]*evaluation.M2ConditionReport) {
			r["basic"].Runner.Metadata.Model = "different"
		}},
		{"unknown report version", func(_ *evidence.Document, r map[string]*evaluation.M2ConditionReport) {
			r["basic"].SchemaVersion = "repocue/evaluation-999"
		}},
		{"mixed report versions", func(_ *evidence.Document, r map[string]*evaluation.M2ConditionReport) {
			r["basic"].SchemaVersion = evaluation.LegacyM2SchemaVersion
			r["basic"].Runner.SchemaVersion = "repocue/evaluation-runner-3"
		}},
		{"mismatched runner version", func(_ *evidence.Document, r map[string]*evaluation.M2ConditionReport) {
			r["basic"].Runner.SchemaVersion = "repocue/evaluation-runner-3"
		}},
		{"benchmark", func(_ *evidence.Document, r map[string]*evaluation.M2ConditionReport) {
			r["basic"].BenchmarkVersion = "future"
		}},
		{"budget", func(_ *evidence.Document, r map[string]*evaluation.M2ConditionReport) { r["basic"].Cue.MaxTokens++ }},
		{"negative cost", func(_ *evidence.Document, r map[string]*evaluation.M2ConditionReport) {
			r["basic"].Baseline.BytesScanned = -1
		}},
		{"producer missing", func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) {
			p.Producer.ExecutableDigest = ""
		}},
		{"duplicate pair", func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) {
			p.Reports = append(p.Reports, p.Reports...)
		}},
		{"wrong condition", func(_ *evidence.Document, r map[string]*evaluation.M2ConditionReport) {
			r["basic"].Condition = "ranked"
		}},
		{"cue command", func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) {
			p.CueCLI[0].Arguments[0] = "status"
		}},
		{"cue trace link", func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) {
			p.CueCLI[0].GitTraceDigest = p.CueCLI[0].OutputDigest
		}},
		{"cue duration", func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) { p.CueCLI[0].DurationMS += 1000 }},
		{"score totals", func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) {
			value := 0
			p.Scoring[0].Passed = &value
		}},
		{"scoring basis", func(p *evidence.Document, _ map[string]*evaluation.M2ConditionReport) {
			p.Scoring[0].Basis.Dirty = true
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			reports, proof := fixtureInputs(t, f, test.edit)
			fixtureFailure(t, f, "calibration", "import", "--repository", f.root, "--cache-dir", f.cache, "--reports", reports, "--evidence", proof)
			if len(fixtureReport(t, f).Profiles) != 0 {
				t.Fatal("failed import published a profile")
			}
		})
	}
	data, err := os.ReadFile(filepath.Join(f.reports, f.manifest.CalibrationEvidence.Reports[0].Path))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.reports, f.manifest.CalibrationEvidence.Reports[0].Path), string(data)+" ")
	fixtureFailure(t, f, "calibration", "import", "--repository", f.root, "--cache-dir", f.cache, "--reports", f.reports, "--evidence", f.proof)
}

func TestReportingCorrectnessRetainsWrongAnswersAndMissingCoverage(t *testing.T) {
	f := prepareReportingFixture(t)
	for _, name := range []string{"branch", "head", "dirty", "tracked", "untracked", "missing", "partial"} {
		t.Run(name, func(t *testing.T) {
			reports, proof := fixtureInputs(t, f, func(p *evidence.Document, reports map[string]*evaluation.M2ConditionReport) {
				if name == "missing" {
					p.Scoring = []evidence.Scoring{}
					return
				}
				if name == "partial" {
					p.Scoring = p.Scoring[:1]
					return
				}
				for _, report := range reports {
					var answer benchmark.Answer
					if err := json.Unmarshal(report.Runner.FinalResponse, &answer); err != nil {
						t.Fatal(err)
					}
					wrong := strings.Repeat("a", 40)
					switch name {
					case "branch":
						wrong = "wrong-branch"
						answer.Git.Branch = &wrong
					case "head":
						answer.Git.Head = &wrong
					case "dirty":
						answer.Git.Dirty = !answer.Git.Dirty
					case "tracked":
						answer.Git.TrackedChanges = []string{"not-a-change"}
					case "untracked":
						answer.Git.Untracked = []string{"not-untracked"}
					}
					data, err := json.Marshal(answer)
					if err != nil {
						t.Fatal(err)
					}
					report.Runner.FinalResponse = data
				}
				for index := range p.Scoring {
					p.Scoring[index].Passed = nil
					p.Scoring[index].Total = nil
				}
			})
			id := fixtureImport(t, f, reports, proof)
			result := fixtureReport(t, f, "--profile", id)
			if len(result.Calibration.Pairs) != 1 || len(result.Projections) == 0 {
				t.Fatal("incorrect answers disappeared from costs")
			}
			if name == "missing" {
				if result.Quality.DirectCorrectness.Count != 0 || result.Quality.BasicCorrectness.Count != 0 {
					t.Fatal("missing score became observed")
				}
			} else if name == "partial" {
				if result.Quality.CorrectnessDifference.Count != 0 || result.Quality.DirectCorrectness.Count+result.Quality.BasicCorrectness.Count != 1 {
					t.Fatal("partial scoring lost coverage")
				}
			} else {
				for _, sample := range []reporting.Sample{result.Calibration.Pairs[0].Direct, result.Calibration.Pairs[0].Basic} {
					if sample.Correctness.Passed == nil || *sample.Correctness.Passed != 4 || *sample.Correctness.Total != 5 || *sample.Correctness.Ratio != 0.8 {
						t.Fatalf("wrong independent score: %+v", sample.Correctness)
					}
				}
			}
		})
	}
	if report := fixtureReport(t, f); report.Calibration != nil || len(report.Projections) != 0 || len(report.Profiles) != 7 {
		t.Fatal("multiple profiles were selected implicitly")
	}
}

func TestReportingCLIStateTransitionsHistoricalCutoffAndInvalidFlags(t *testing.T) {
	f := prepareReportingFixture(t)
	initial := fixtureReport(t, f)
	if initial.Calibration != nil || len(initial.Projections) != 0 || initial.Window.Days != 30 {
		t.Fatal("no-profile defaults differ")
	}
	for _, extra := range [][]string{{"--window", "0d"}, {"--window", "-1d"}, {"--window", "x"}, {"--window", "999999999d"}, {"--consumers", "0"}, {"--consumers=-1"}, {"--consumers", "9007199254740993"}, {"--as-of", "2099-01-01T00:00:00Z"}, {"--as-of", "2020-01-01T00:00:00-08:00"}, {"--format", "xml"}, {"--profile", "missing"}} {
		fixtureFailure(t, f, append([]string{"report", "--repository", f.root, "--cache-dir", f.cache}, extra...)...)
	}
	fixtureCLI(t, f, "rebaseline", "--repository", f.root, "--cache-dir", f.cache, "--label", "binding-test")
	id := fixtureImport(t, f, f.reports, f.proof)
	bound := fixtureReport(t, f, "--profile", id)
	if bound.Calibration.Binding.Snapshot == bound.Calibration.Pairs[0].Direct.SourceSnapshot {
		t.Fatal("evaluation snapshot was used as local identity")
	}
	cutoff := time.Now().UTC().Format(time.RFC3339Nano)
	historical := fixtureReport(t, f, "--profile", id, "--as-of", cutoff)
	writeFile(t, filepath.Join(f.root, "README.md"), "changed tracked documentation\n")
	stale := fixtureReport(t, f, "--profile", id)
	if stale.Freshness != "refresh-needed" || len(stale.Projections) != 0 {
		t.Fatal("stale tracked state projected savings")
	}
	fixtureCLI(t, f, "refresh", "--repository", f.root, "--cache-dir", f.cache)
	estimated := fixtureReport(t, f, "--profile", id)
	if estimated.Applicability.Status != "estimated" || len(estimated.Projections) == 0 {
		t.Fatalf("compatible snapshot not estimated: %+v", estimated.Applicability)
	}
	again := fixtureReport(t, f, "--profile", id, "--as-of", cutoff)
	if !reflect.DeepEqual(historical.State, again.State) || !reflect.DeepEqual(historical.Projections, again.Projections) || !reflect.DeepEqual(historical.Applicability, again.Applicability) {
		t.Fatal("live change replaced historical reference")
	}
	fixtureCLI(t, f, "rebaseline", "--repository", f.root, "--cache-dir", f.cache, "--label", "next-epoch")
	rebased := fixtureReport(t, f, "--profile", id)
	if rebased.Applicability.Status != "not_applicable" || len(rebased.Projections) != 0 || len(rebased.Activity) != 3 {
		t.Fatalf("epoch boundary lost: %+v", rebased.Applicability)
	}
	before := fixtureReport(t, f, "--as-of", initial.State.ObservedAt.Format(time.RFC3339Nano))
	if before.State != nil || before.Calibration != nil {
		t.Fatal("strict cutoff included state observed at cutoff")
	}
}

type reportingFixture struct {
	binary, root, cache, external, reports, proof, answer, delivered string
	manifest                                                         evaluation.M2Manifest
}

func prepareReportingFixture(t *testing.T) reportingFixture {
	return prepareReportingFixtureWithSetup(t, nil)
}

func prepareReportingFixtureWithSetup(t *testing.T, setup func(string)) reportingFixture {
	t.Helper()
	fixture := reportingFixture{root: createGitRepository(t), cache: t.TempDir(), external: t.TempDir()}
	if setup != nil {
		setup(fixture.root)
	}
	fixture.binary = filepath.Join(t.TempDir(), "repocue")
	build := exec.Command("go", "build", "-o", fixture.binary, "./cmd/repocue")
	build.Dir = filepath.Join("..", "..")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, output)
	}
	fixture.answer = filepath.Join(fixture.external, "answer.json")
	fixture.delivered = filepath.Join(fixture.external, "delivered-digest")
	repo, err := repository.Open(context.Background(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := repo.CaptureEvidence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	answer := benchmark.Answer{SchemaVersion: benchmark.AnswerSchemaVersion, ProjectPurpose: "PRIVATE_FIXTURE_RESPONSE_TEXT",
		Git:                benchmark.GitState{Branch: capture.Scan.Basis.Branch, Head: capture.Scan.Basis.Head, Dirty: capture.Scan.Basis.Dirty, TrackedChanges: []string{}, Untracked: []string{}},
		PrimaryEntryPoints: []benchmark.EntryPoint{}, MajorComponents: []benchmark.Component{}, ImportantDocumentation: []benchmark.Document{}, RecentRelevantChanges: []string{}, ProjectSymbols: []benchmark.ProjectSymbol{}, Uncertainties: []string{fixture.root}}
	encoded, err := json.Marshal(answer)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, fixture.answer, string(encoded))
	t.Setenv("REPOCUE_TEST_ANSWER_FILE", fixture.answer)
	t.Setenv("REPOCUE_TEST_DELIVERED_DIGEST", fixture.delivered)
	runner := filepath.Join(fixture.external, "runner")
	testExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	quoted := "'" + strings.ReplaceAll(testExecutable, "'", "'\\''") + "'"
	writeFile(t, runner, "#!/bin/sh\nREPOCUE_TEST_AGENT_BOUNDARY=1\nexport REPOCUE_TEST_AGENT_BOUNDARY\nexec "+quoted+" -test.run=^TestReportingRunnerBoundary$\n")
	if err := os.Chmod(runner, 0o700); err != nil {
		t.Fatal(err)
	}
	fixtureCLI(t, fixture, "init", "--cache-dir", fixture.cache, fixture.root)
	output := fixtureCLI(t, fixture, "evaluate-m2", "--repository", fixture.root, "--conditions", "direct,basic", "--max-tokens", "500", "--runner", runner,
		"--task-file", fixture.answer, "--output-directory", filepath.Join(fixture.external, "sets"))
	if err := json.Unmarshal(output, &fixture.manifest); err != nil {
		t.Fatal(err)
	}
	sets, err := os.ReadDir(filepath.Join(fixture.external, "sets"))
	if err != nil || len(sets) != 1 {
		t.Fatalf("report set: %v %v", sets, err)
	}
	fixture.reports = filepath.Join(fixture.external, "sets", sets[0].Name())
	fixture.proof = filepath.Join(fixture.reports, "calibration-evidence.json")
	return fixture
}

func TestReportingUntrackedContentMismatchSurvivesOrdinaryRefresh(t *testing.T) {
	f := prepareReportingFixtureWithSetup(t, func(root string) { writeFile(t, filepath.Join(root, "untracked.txt"), "first") })
	id := fixtureImport(t, f, f.reports, f.proof)
	before := fixtureReport(t, f, "--profile", id)
	if before.Applicability.Status != "measured_reference" || before.Freshness != "dirty-but-indexed" {
		t.Fatal("dirty initial evidence is not applicable")
	}
	writeFile(t, filepath.Join(f.root, "untracked.txt"), "other")
	for index := 0; index < 2; index++ {
		if index == 1 {
			fixtureCLI(t, f, "refresh", "--repository", f.root, "--cache-dir", f.cache)
		}
		after := fixtureReport(t, f, "--profile", id)
		if after.State.Snapshot != before.State.Snapshot || after.Freshness != before.Freshness || after.Applicability.Status != "not_applicable" || len(after.Projections) != 0 {
			t.Fatal("same-path untracked change was ignored or incorrectly repaired by refresh")
		}
		fixtureFailure(t, f, "calibration", "import", "--repository", f.root, "--cache-dir", f.cache, "--reports", f.reports, "--evidence", f.proof)
	}
	t.Setenv("REPOCUE_TEST_MUTATE_FILE", filepath.Join(f.root, "untracked.txt"))
	output := filepath.Join(f.external, "failed-set")
	fixtureFailure(t, f, "evaluate-m2", "--repository", f.root, "--conditions", "direct,basic", "--runner", filepath.Join(f.external, "runner"), "--task-file", f.answer, "--output-directory", output)
	entries, err := os.ReadDir(output)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("persistent agent mutation published a report set")
	}
}

func TestReportingOptionalAssessmentsHaveExactJoinsAndCoverage(t *testing.T) {
	f := prepareReportingFixture(t)
	for _, mode := range []string{"none", "partial", "complete", "unknown date", "duplicate", "wrong head", "mixed assessor", "bad digest", "wrong response"} {
		t.Run(mode, func(t *testing.T) {
			assessments := t.TempDir()
			reports, proofPath := fixtureInputs(t, f, func(p *evidence.Document, reports map[string]*evaluation.M2ConditionReport) {
				p.Assessments = []evidence.AssessmentReference{}
				if mode == "none" {
					return
				}
				for index, ref := range p.Reports {
					if mode == "partial" && index > 0 {
						break
					}
					report := reports[ref.Condition]
					responseID := "response-" + ref.Condition
					scores := map[string]int{"purpose": 3, "architecture": 2, "documentation": 3, "recent_changes": 2, "structural_completeness": 1, "uncertainty_handling": 4}
					doc := map[string]any{"schema_version": "repocue/qualitative-assessment-2", "assessor": "fixture-reviewer", "response_id": responseID,
						"condition": ref.Condition, "repository_head": report.Repository.Head, "state_fingerprint": ref.StateFingerprint, "run_index": ref.RunIndex, "scores": scores, "evidence": []string{"PRIVATE_ASSESSMENT_NARRATIVE"}}
					if mode == "wrong head" {
						doc["repository_head"] = "wrong"
					}
					if mode == "mixed assessor" && index == 1 {
						doc["assessor"] = "another-reviewer"
					}
					if mode == "wrong response" {
						doc["response_id"] = "wrong-response"
					}
					data, err := json.Marshal(doc)
					if err != nil {
						t.Fatal(err)
					}
					path := responseID + ".json"
					writeFile(t, filepath.Join(assessments, path), string(data))
					item := evidence.AssessmentReference{FileReference: evidence.FileReference{Path: path, Digest: evidence.Digest(data)}, ReportDigest: ref.Digest, ResponseID: responseID}
					if mode != "unknown date" {
						now := time.Now().UTC()
						item.AssessedAt = &now
					}
					if mode == "bad digest" {
						item.Digest = evidence.Digest([]byte("wrong"))
					}
					p.Assessments = append(p.Assessments, item)
				}
				if mode == "duplicate" {
					p.Assessments = append(p.Assessments, p.Assessments[0])
				}
			})
			valid := mode == "none" || mode == "partial" || mode == "complete" || mode == "unknown date"
			if !valid {
				fixtureFailure(t, f, "calibration", "import", "--repository", f.root, "--cache-dir", f.cache, "--reports", reports, "--evidence", proofPath, "--assessments", assessments)
				return
			}
			id := fixtureImport(t, f, reports, proofPath, "--assessments", assessments)
			report := fixtureReport(t, f, "--profile", id)
			for _, stat := range report.Quality.Qualitative {
				if (mode == "complete" || mode == "unknown date") && (stat.Count != 1 || stat.Median == nil || *stat.Median != 0) {
					t.Fatalf("changed scores: %+v", stat)
				}
				if (mode == "none" || mode == "partial") && (stat.Count != 0 || stat.Unknown != 1) {
					t.Fatalf("invented pair quality: %+v", stat)
				}
			}
			if mode == "unknown date" && report.Calibration.Pairs[0].Direct.Assessment.AssessedAt != nil {
				t.Fatal("unknown date was invented")
			}
			data, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte("PRIVATE_ASSESSMENT_NARRATIVE")) {
				t.Fatal("assessment narrative persisted")
			}
		})
	}
}

func fixtureCLI(t *testing.T, fixture reportingFixture, args ...string) []byte {
	t.Helper()
	command := exec.Command(fixture.binary, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("repocue %v: %v: %s", args, err, stderr.String())
	}
	return stdout.Bytes()
}

func fixtureReport(t *testing.T, fixture reportingFixture, extra ...string) reporting.Report {
	t.Helper()
	args := append([]string{"report", "--repository", fixture.root, "--cache-dir", fixture.cache}, extra...)
	output := fixtureCLI(t, fixture, args...)
	if err := schema.Validate("savings-report-v1.schema.json", output); err != nil {
		t.Fatal(err)
	}
	var report reporting.Report
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func TestReportingCLIImportsRealEvidenceAndPreservesRepository(t *testing.T) {
	fixture := prepareReportingFixture(t)
	delivered, err := os.ReadFile(fixture.delivered)
	if err != nil {
		t.Fatal(err)
	}
	if string(delivered) != fixture.manifest.CalibrationEvidence.CueCLI[0].OutputDigest {
		t.Fatal("assisted agent received a different cue")
	}
	first := fixtureCLI(t, fixture, "calibration", "import", "--repository", fixture.root, "--cache-dir", fixture.cache, "--reports", fixture.reports, "--evidence", fixture.proof)
	second := fixtureCLI(t, fixture, "calibration", "import", "--repository", fixture.root, "--cache-dir", fixture.cache, "--reports", fixture.reports, "--evidence", fixture.proof)
	if !bytes.Equal(first, second) {
		t.Fatalf("import is not idempotent: %s / %s", first, second)
	}
	report := fixtureReport(t, fixture)
	if report.Applicability.Status != "measured_reference" || report.Calibration == nil || len(report.Calibration.Pairs) != 1 {
		t.Fatalf("wrong applicability: %+v", report.Applicability)
	}
	if report.Quality.DirectCorrectness.Median == nil || *report.Quality.DirectCorrectness.Median != 1 || report.Quality.Status != "not_observed" {
		t.Fatalf("wrong independent quality: %+v", report.Quality)
	}
	found := false
	for _, projection := range report.Projections {
		if projection.Metric == "total_tokens" && projection.Consumers == 10 {
			found = true
			if projection.Saving.Median == nil || *projection.Saving.Median != 9000 {
				t.Fatalf("cue tokens double counted: %+v", projection)
			}
		}
		if projection.Metric == "git_invocations" && projection.Saving.Median != nil {
			t.Fatal("classified Git commands were treated as observed Git processes")
		}
	}
	if !found {
		t.Fatal("missing fixed reuse scenario")
	}
	pair := report.Calibration.Pairs[0]
	t.Logf("Fixture-only CLI measurement: baseline %.3f ms, update %.3f ms, cue %.3f ms, cue %.0f bytes / %.0f estimated tokens; agent token values are synthetic",
		*pair.Baseline["execution_duration_ms"].Value, *pair.Update["execution_duration_ms"].Value,
		*pair.Cue["execution_duration_ms"].Value, *pair.Cue["output_bytes"].Value, *pair.Cue["estimated_tokens"].Value)
	for name, metric := range report.Unavailable {
		if metric.Status != "not_observed" || metric.Value != nil {
			t.Fatalf("invented metric %s: %+v", name, metric)
		}
	}
	serialized, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"PRIVATE_FIXTURE_RESPONSE_TEXT", fixture.root, fixture.external} {
		if bytes.Contains(serialized, []byte(forbidden)) {
			t.Fatalf("report retained private source text: %s", forbidden)
		}
	}
	before := fixtureCLI(t, fixture, "metrics", "--repository", fixture.root, "--cache-dir", fixture.cache)
	fixtureReport(t, fixture, "--consumers", "7")
	after := fixtureCLI(t, fixture, "metrics", "--repository", fixture.root, "--cache-dir", fixture.cache)
	if !bytes.Equal(before, after) {
		t.Fatal("report wrote maintenance or cue metrics")
	}
	git := exec.Command("git", "-C", fixture.root, "--no-optional-locks", "status", "--porcelain")
	status, err := git.Output()
	if err != nil || len(status) != 0 {
		t.Fatalf("repository changed: %s %v", status, err)
	}

	cutoff := time.Now().UTC().Format(time.RFC3339Nano)
	jsonBytes := fixtureCLI(t, fixture, "report", "--repository", fixture.root, "--cache-dir", fixture.cache, "--as-of", cutoff)
	markdown := fixtureCLI(t, fixture, "report", "--repository", fixture.root, "--cache-dir", fixture.cache, "--as-of", cutoff, "--format", "markdown")
	start := bytes.Index(markdown, []byte("```json\n"))
	end := bytes.LastIndex(markdown, []byte("\n```"))
	if start < 0 || end < start {
		t.Fatal("Markdown lacks complete calculation result")
	}
	var left, right map[string]any
	if err := json.Unmarshal(jsonBytes, &left); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(markdown[start+8:end], &right); err != nil {
		t.Fatal(err)
	}
	for _, value := range []map[string]any{left, right} {
		delete(value, "generated_at")
		delete(value, "live_evidence")
		delete(value, "database_size_observed_at")
	}
	l, _ := json.Marshal(left)
	r, _ := json.Marshal(right)
	if !bytes.Equal(l, r) {
		t.Fatal("JSON and Markdown use different historical calculation results")
	}
}
