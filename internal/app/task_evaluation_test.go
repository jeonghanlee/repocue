package app_test

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/docs/schema"
	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/evaluation"
	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

type taskEvaluationFixture struct {
	taskFixture
	runner, provider, taskFile, answerFile, ratesFile, log string
	spec                                                   taskbenchmark.Spec
}

func newTaskEvaluationFixture(t *testing.T, binary, runner, provider, kind string) taskEvaluationFixture {
	t.Helper()
	f := taskEvaluationFixture{taskFixture: newTaskFixture(t, binary), runner: runner, provider: provider}
	for _, name := range []string{"go.mod", "verify.go", "verify_test.go"} {
		value, err := os.ReadFile(filepath.Join("testdata", "task-context", name))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(f.root, name), string(value))
	}
	runGit(t, f.root, "add", "go.mod", "verify.go", "verify_test.go")
	f.cli(t, "refresh", "--repository", f.root, "--cache-dir", f.cache)
	f.log = filepath.Join(f.external, "exec.log")
	f.taskFile = filepath.Join(f.external, "benchmark-input.json")
	f.answerFile = filepath.Join(f.external, "answer-input.json")
	f.ratesFile = filepath.Join(f.external, "rate-input.json")
	f.spec = taskbenchmark.Spec{SchemaVersion: taskbenchmark.Version, ID: "fixture-" + kind, TaskKind: kind,
		ContextFile: "docs/context.json", Targets: []string{"cmd"}, MaxTokens: 5000,
		Prompt: "Inspect the declared original context and complete the scoped task.",
		Policy: &taskbenchmark.Policy{ID: "fixture-policy", RequiredPairs: 1,
			Rubric: "Original rule and check references must match the frozen task.", Checks: []taskbenchmark.Check{}}}
	if kind == "verification" {
		f.spec.Policy.Checks = []taskbenchmark.Check{{ID: "real-go-test", Command: "go test ./...", ExpectedExit: 0, OutputContains: []string{"ok"}}}
	}
	args := f.taskArgs(5000, "cmd")
	for i := range args {
		if args[i] == "--task-kind" {
			args[i+1] = kind
		}
	}
	expected, err := cue.DecodeTask(f.cli(t, args...))
	if err != nil {
		t.Fatal(err)
	}
	sources, err := taskbenchmark.Sources(expected)
	if err != nil {
		t.Fatal(err)
	}
	answer := taskbenchmark.Answer{SchemaVersion: taskbenchmark.AnswerVersion, TaskID: f.spec.ID, TaskKind: kind,
		RequiredIDs: expected.Content.RequiredIDs, SourceRefs: []taskcontext.Reference{},
		Claims: []taskbenchmark.Claim{}, Checks: []string{"real-go-test"}, Uncertainties: []string{}}
	for _, source := range sources {
		answer.SourceRefs = append(answer.SourceRefs, taskcontext.Reference{Path: source.Path, ContentDigest: source.ContentDigest, Lines: source.Lines})
	}
	if kind == "explanation" {
		answer.Claims = []taskbenchmark.Claim{{Text: "The original context requires preserving original sources and running the declared verification.",
			SourceRefs: answer.SourceRefs}}
	}
	taskWriteJSON(t, f.taskFile, f.spec)
	taskWriteJSON(t, f.answerFile, answer)
	taskWriteJSON(t, f.ratesFile, taskbenchmark.Rates{ID: "local-fixture-only", AsOf: "2026-10-07",
		Unit: "credits_per_million_tokens", Model: "fixture-model", SpeedTier: "standard", Input: 1, CachedInput: 0.1, Output: 2})
	t.Setenv("REPOCUE_TEST_EXEC_LOG", f.log)
	t.Setenv("REPOCUE_TEST_ANSWER_FILE", f.answerFile)
	t.Setenv("REPOCUE_TEST_MODE", "")
	t.Setenv("REPOCUE_TEST_FAIL_ARM", "")
	t.Setenv("REPOCUE_TEST_USAGE_JSON", "")
	return f
}

func taskWriteJSON(t *testing.T, name string, value any) {
	t.Helper()
	serialized, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, name, string(serialized))
}

func (f taskEvaluationFixture) evalArgs(runner bool) []string {
	args := []string{"evaluate-task", "--repository", f.root, "--cache-dir", f.cache,
		"--task-file", f.taskFile, "--output-directory", f.external, "--rates-file", f.ratesFile}
	if runner {
		args = append(args, "--runner", f.runner, "--codex-binary", f.provider, "--model", "fixture-model", "--reasoning-effort", "low")
	}
	return args
}

func (f taskEvaluationFixture) run(t *testing.T, extra ...string) (evaluation.TaskReport, error) {
	t.Helper()
	stdout, stderr, err := f.raw(append(f.evalArgs(true), extra...)...)
	var report evaluation.TaskReport
	if decodeErr := taskcontext.DecodeStrict(stdout, "task-evaluation-v1.schema.json#/$defs/report", &report); decodeErr != nil {
		t.Fatalf("real report contract: %v: %s %s", decodeErr, stdout, stderr)
	}
	return report, err
}

func (f taskEvaluationFixture) summary(t *testing.T, extra ...string) evaluation.TaskSummary {
	t.Helper()
	value := f.cli(t, append([]string{"evaluate-task", "summary", "--input-directory", f.external}, extra...)...)
	var result evaluation.TaskSummary
	if err := taskcontext.DecodeStrict(value, "task-evaluation-v1.schema.json#/$defs/summary", &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestTaskEvaluationShippedPublicExamplesAndNoPolicy(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	runner := buildTaskExecutable(t, "./cmd/repocue-codex-runner")
	provider := buildTaskExecutable(t, "./internal/app/testdata/task-context/agent.go")
	f := newTaskEvaluationFixture(t, binary, runner, provider, "discovery")
	for _, relative := range []string{"README.md", "docs/task-context.example.json"} {
		value, err := os.ReadFile(filepath.Join("..", "..", relative))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(f.root, relative), string(value))
	}
	runGit(t, f.root, "add", "README.md", "docs/task-context.example.json")
	runGit(t, f.root, "commit", "-m", "Track public fixture inputs")
	f.cli(t, "refresh", "--repository", f.root, "--cache-dir", f.cache)
	// Each documentation command uses the same clean checkout and actual CLI.
	t.Setenv("XDG_CACHE_HOME", f.cache)
	command := exec.Command(binary, "init", ".")
	command.Dir = f.root
	if value, err := command.CombinedOutput(); err != nil {
		t.Fatalf("documented init: %v %s", err, value)
	}
	command = exec.Command(binary, "cue", "--view", "task", "--context-file", "docs/task-context.example.json",
		"--task-kind", "discovery", "--target", "internal/cue", "--max-tokens", "5000")
	command.Dir = f.root
	value, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	example, err := cue.DecodeTask(value)
	if err != nil || len(example.Content.RequiredIDs) != 1 || example.Content.RequiredIDs[0] != "repository-context" {
		t.Fatal("documented task cue example failed")
	}
	for _, kind := range []string{"explanation", "discovery", "verification"} {
		name, err := filepath.Abs(filepath.Join("..", "..", "docs", "benchmarks", "task-context-v1", kind+".json"))
		if err != nil {
			t.Fatal(err)
		}
		command = exec.Command(binary, "evaluate-task", "--task-file", name, "--output-directory", f.external)
		command.Dir = f.root
		value, err := command.Output()
		if err != nil {
			t.Fatalf("public preparation %s: %v", kind, err)
		}
		if err := schema.Validate("task-evaluation-v1.schema.json", value); err != nil {
			t.Fatal(err)
		}
	}
	command = exec.Command(binary, "evaluate-task", "summary", "--input-directory", f.external)
	command.Dir = f.root
	if value, err := command.Output(); err != nil || schema.Validate("task-evaluation-v1.schema.json", value) != nil {
		t.Fatal("documented summary failed")
	}
	if len(taskExecutions(t, f.log)) != 0 {
		t.Fatal("public preparation examples invoked a provider")
	}
	// Missing frozen policy and missing rates independently suppress a verdict.
	f.spec.Policy = nil
	taskWriteJSON(t, f.taskFile, f.spec)
	report, err := f.run(t, "--rates-file", "")
	if err != nil {
		t.Fatal(err)
	}
	summary := f.summary(t)
	if len(summary.Attempts) != 2 || summary.Groups[0].Verdict.Value != nil ||
		summary.Attempts[0].Metrics["credit_estimate"].Value != nil {
		t.Fatal("absent policy/rates awarded a verdict")
	}
	journal := filepath.Join(report.Directory, report.Attempts[1].ID, "attempt.json")
	if err := os.Rename(journal, journal+".retained"); err != nil {
		t.Fatal(err)
	}
	interrupted := f.summary(t)
	if len(interrupted.Attempts) != 2 || interrupted.Groups[0].Verdict.Value != nil {
		t.Fatal("incomplete attempt journal hidden")
	}
	if err := os.WriteFile(journal, []byte("{invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	invalid := f.summary(t)
	if len(invalid.Attempts) != 2 || len(invalid.Errors) == 0 || invalid.Groups[0].Verdict.Value != nil {
		t.Fatal("unreadable attempt disappeared from inventory")
	}
}

func TestTaskEvaluationShippedMatchedDifferencesAndCoverage(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	runner := buildTaskExecutable(t, "./cmd/repocue-codex-runner")
	provider := buildTaskExecutable(t, "./internal/app/testdata/task-context/agent.go")
	f := newTaskEvaluationFixture(t, binary, runner, provider, "discovery")
	f.spec.Policy.RequiredPairs = 3
	taskWriteJSON(t, f.taskFile, f.spec)
	for i, counters := range [][2]int64{{100, 90}, {200, 1}, {300, 299}} {
		serialized, _ := json.Marshal(map[string][]int64{"overview": {counters[0], 0, 0}, "task": {counters[1], 0, 0}})
		t.Setenv("REPOCUE_TEST_USAGE_JSON", string(serialized))
		order := "overview,task"
		if i == 1 {
			order = "task,overview"
		}
		if _, err := f.run(t, "--order", order); err != nil {
			t.Fatal(err)
		}
		if i < 2 && f.summary(t).Groups[0].Verdict.Value != nil {
			t.Fatal("partial policy coverage awarded verdict")
		}
	}
	summary := f.summary(t)
	group := summary.Groups[0]
	input := group.Metrics["input_tokens"]
	if len(summary.Attempts) != 6 || input.KnownPairs != 3 || input.UnknownPairs != 0 ||
		input.MedianDifference == nil || *input.MedianDifference != 10 ||
		*input.MinDifference != 1 || *input.MaxDifference != 199 ||
		group.OverviewFirst != 2 || group.TaskFirst != 1 || group.Verdict.Value == nil {
		t.Fatalf("actual matched differences and order counts: %+v", summary)
	}
	t.Setenv("REPOCUE_TEST_MODE", "failure")
	t.Setenv("REPOCUE_TEST_FAIL_ARM", "overview")
	if _, err := f.run(t); err == nil {
		t.Fatal("extra failed pair reported success")
	}
	summary = f.summary(t)
	if len(summary.Attempts) != 7 || summary.Groups[0].Verdict.Value != nil ||
		summary.Groups[0].Metrics["input_tokens"].UnknownPairs != 1 {
		t.Fatal("extra failed attempt hidden from frozen planned coverage")
	}
}

func taskExecutions(t *testing.T, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(raw))
}

func TestTaskEvaluationShippedPreparationPairAndSummary(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	runner := buildTaskExecutable(t, "./cmd/repocue-codex-runner")
	provider := buildTaskExecutable(t, "./internal/app/testdata/task-context/agent.go")
	f := newTaskEvaluationFixture(t, binary, runner, provider, "discovery")
	prepared := f.cli(t, f.evalArgs(false)...)
	if err := schema.Validate("task-evaluation-v1.schema.json", prepared); err != nil {
		t.Fatal(err)
	}
	if len(taskExecutions(t, f.log)) != 0 {
		t.Fatal("preparation launched an agent")
	}
	report, err := f.run(t)
	if err != nil || report.Status != "completed" || len(report.Attempts) != 2 {
		t.Fatalf("pair: %v %+v", err, report)
	}
	for _, attempt := range report.Attempts {
		if !attempt.Eligible || !attempt.Delivery.Complete || attempt.Correctness.Value == nil || !*attempt.Correctness.Value {
			t.Fatalf("actual delivery/selector correctness unavailable: %+v", attempt)
		}
		root := filepath.Join(report.Directory, attempt.ID)
		prompt, err := os.ReadFile(filepath.Join(root, "prompt.txt"))
		if err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(filepath.Join(root, "outer-stdin.txt"))
		if err != nil || !bytes.Equal(actual, prompt) {
			t.Fatal("provider stdin differs from captured prompt")
		}
		if attempt.Metrics["reasoning_output_tokens"].Value != nil || attempt.Metrics["credit_estimate"].Value == nil {
			t.Fatal("missing reasoning changed otherwise complete credit estimate")
		}
		for _, name := range []string{"provider.jsonl", "provider.stderr", "runner.stdout", "runner.stderr", "request.json", "attempt-start.json", "attempt.json"} {
			if _, err := os.Stat(filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	count := len(taskExecutions(t, f.log))
	summary := f.summary(t)
	if len(taskExecutions(t, f.log)) != count || count != 2 {
		t.Fatal("summary or pair launched extra agent")
	}
	if len(summary.Attempts) != 2 || len(summary.Groups) != 1 || len(summary.Errors) != 0 {
		t.Fatalf("inventory: %+v", summary)
	}
	group := summary.Groups[0]
	stdout, _, importErr := f.raw("calibration", "import", "--repository", f.root, "--cache-dir", f.cache,
		"--reports", report.Directory, "--evidence", filepath.Join(report.Directory, "manifest.json"))
	if importErr == nil || len(stdout) != 0 {
		t.Fatal("task result entered legacy calibration importer")
	}
	credit := group.Metrics["credit_estimate"]
	if credit.KnownPairs != 1 || credit.UnknownPairs != 0 || credit.MedianDifference == nil ||
		math.Abs(*credit.MedianDifference-0.00032) > 1e-12 || group.Verdict.Value == nil || !*group.Verdict.Value {
		t.Fatalf("local fixture paired credit calculation: %+v", group)
	}
	if group.Metrics["cpu_ms"].KnownPairs != 0 || group.Metrics["task_ready_ms"].UnknownPairs != 1 ||
		group.Metrics["cue_bytes"].KnownPairs != 1 {
		t.Fatal("unmeasured values or cue costs misclassified")
	}
	before := len(taskExecutions(t, f.log))
	f.cli(t, "evaluate-task", "summary", "--input-directory", f.external)
	if len(taskExecutions(t, f.log)) != before {
		t.Fatal("repeated summary launched a process")
	}
	raw, err := os.ReadFile(filepath.Join(report.Directory, report.Attempts[1].ID, "submitted-prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(report.Directory, report.Attempts[1].ID, "submitted-prompt.txt"), append(raw, 'x'), 0600); err != nil {
		t.Fatal(err)
	}
	corrupted := f.summary(t)
	if corrupted.Groups[0].Verdict.Value != nil || corrupted.Groups[0].Metrics["credit_estimate"].KnownPairs != 0 {
		t.Fatal("changed retained prompt still awarded comparison")
	}
}

func TestTaskEvaluationShippedFrozenSchemaAndDirectoryIdentity(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	runner := buildTaskExecutable(t, "./cmd/repocue-codex-runner")
	provider := buildTaskExecutable(t, "./internal/app/testdata/task-context/agent.go")
	f := newTaskEvaluationFixture(t, binary, runner, provider, "discovery")
	report, err := f.run(t)
	if err != nil || report.OutputSchemaDigest == nil || len(report.Attempts) != 2 {
		t.Fatalf("schema pair: %v %+v", err, report)
	}
	read := func(name string) []byte {
		t.Helper()
		value, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	pairSchema := read(filepath.Join(report.Directory, "answer.schema.json"))
	if taskcontext.Digest(pairSchema) != *report.OutputSchemaDigest {
		t.Fatal("pair schema was not frozen from actual bytes")
	}
	for _, attempt := range report.Attempts {
		root := filepath.Join(report.Directory, attempt.ID)
		if attempt.Transport.OutputSchemaDigest == nil || *attempt.Transport.OutputSchemaDigest != *report.OutputSchemaDigest ||
			!bytes.Equal(read(filepath.Join(root, "answer.schema.json")), pairSchema) ||
			!bytes.Equal(read(filepath.Join(root, "outer-schema.json")), pairSchema) {
			t.Fatal("actual provider schema differs from frozen pair/captured schema")
		}
	}
	absolute := f.cli(t, "evaluate-task", "summary", "--input-directory", f.external)
	for _, mode := range []string{"relative", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			input := filepath.Base(f.external)
			if mode == "symlink" {
				input = filepath.Join(t.TempDir(), "evidence-alias")
				if err := os.Symlink(f.external, input); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command(binary, "evaluate-task", "summary", "--input-directory", input)
			command.Dir = filepath.Dir(f.external)
			output, err := command.Output()
			if err != nil || !bytes.Equal(output, absolute) {
				t.Fatalf("same physical summary differs for %s: %v: %s", mode, err, output)
			}
		})
	}
	attemptRoot := filepath.Join(report.Directory, report.Attempts[1].ID)
	for _, mode := range []string{"pair-changed", "pair-missing", "capture-changed", "request-digest", "request-path"} {
		t.Run(mode, func(t *testing.T) {
			name := filepath.Join(report.Directory, "answer.schema.json")
			if mode == "capture-changed" {
				name = filepath.Join(attemptRoot, "answer.schema.json")
			} else if strings.HasPrefix(mode, "request-") {
				name = filepath.Join(attemptRoot, "request.json")
			}
			original := read(name)
			recordName := filepath.Join(attemptRoot, "attempt.json")
			originalRecord := read(recordName)
			t.Cleanup(func() {
				writeFile(t, name, string(original))
				writeFile(t, recordName, string(originalRecord))
			})
			if mode == "pair-missing" {
				if err := os.Rename(name, name+".unavailable"); err != nil {
					t.Fatal(err)
				}
			} else {
				changed := []byte("{}\n")
				if strings.HasPrefix(mode, "request-") {
					var request taskbenchmark.RunnerRequest
					if err := json.Unmarshal(original, &request); err != nil {
						t.Fatal(err)
					}
					if mode == "request-digest" {
						request.OutputSchemaDigest = taskcontext.Digest(changed)
					} else {
						request.OutputSchemaFile = filepath.Join(attemptRoot, "answer.schema.json")
					}
					changed, err = json.Marshal(request)
					if err != nil {
						t.Fatal(err)
					}
				}
				writeFile(t, name, string(changed))
				if mode != "pair-changed" {
					var attempt evaluation.TaskAttempt
					if err := json.Unmarshal(originalRecord, &attempt); err != nil {
						t.Fatal(err)
					}
					attempt.Artifacts[filepath.Base(name)] = taskcontext.Digest(changed)
					taskWriteJSON(t, recordName, attempt)
				}
			}
			corrupted := f.summary(t)
			if len(corrupted.Attempts) != 2 || len(corrupted.Groups) != 1 ||
				corrupted.Groups[0].Verdict.Value != nil || corrupted.Groups[0].Metrics["credit_estimate"].KnownPairs != 0 {
				t.Fatalf("changed schema input still awarded comparison or lost attempts: %+v", corrupted)
			}
		})
	}
	if restored := f.cli(t, "evaluate-task", "summary", "--input-directory", f.external); !bytes.Equal(restored, absolute) {
		t.Fatal("restored actual evidence differs from original summary")
	}
	if len(taskExecutions(t, f.log)) != 2 {
		t.Fatal("schema/path validation launched another provider")
	}
}

func TestTaskEvaluationShippedSchemaMutationBeforeSecondArm(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	runner := buildTaskExecutable(t, "./cmd/repocue-codex-runner")
	provider := buildTaskExecutable(t, "./internal/app/testdata/task-context/agent.go")
	f := newTaskEvaluationFixture(t, binary, runner, provider, "discovery")
	t.Setenv("REPOCUE_TEST_MODE", "schema-mutation")
	t.Setenv("REPOCUE_TEST_FAIL_ARM", "overview")
	report, err := f.run(t)
	if err == nil || len(report.Attempts) != 2 || report.Attempts[0].Status != "completed" ||
		report.Attempts[1].Status != "failed" || len(taskExecutions(t, f.log)) != 1 {
		t.Fatalf("changed schema was delivered or attempts were lost: %v %+v", err, report)
	}
	if report.Attempts[1].Transport == nil || report.Attempts[1].Transport.Error == nil ||
		!strings.Contains(*report.Attempts[1].Transport.Error, "answer schema digest mismatch") {
		t.Fatal("second arm did not reject actual changed schema before provider launch")
	}
	summary := f.summary(t)
	if len(summary.Attempts) != 2 || len(summary.Groups) != 1 || summary.Groups[0].Verdict.Value != nil ||
		summary.Groups[0].Metrics["credit_estimate"].KnownPairs != 0 {
		t.Fatal("changed frozen schema still produced a complete comparison")
	}
}

func TestTaskEvaluationShippedFailuresAndUnavailableEvidence(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	runner := buildTaskExecutable(t, "./cmd/repocue-codex-runner")
	provider := buildTaskExecutable(t, "./internal/app/testdata/task-context/agent.go")
	for _, test := range []struct {
		mode, arm string
		attempts  int
		failure   bool
	}{
		{"failure", "overview", 1, true}, {"failure", "task", 2, true}, {"malformed", "task", 2, true},
		{"mutation", "task", 2, true}, {"basis-unavailable", "task", 2, true},
		{"missing-counter", "task", 2, false}, {"wrong-answer", "task", 2, false},
	} {
		t.Run(test.mode+"/"+test.arm, func(t *testing.T) {
			f := newTaskEvaluationFixture(t, binary, runner, provider, "discovery")
			t.Setenv("REPOCUE_TEST_MODE", test.mode)
			t.Setenv("REPOCUE_TEST_FAIL_ARM", test.arm)
			report, err := f.run(t)
			if (err != nil) != test.failure || len(report.Attempts) != test.attempts ||
				len(taskExecutions(t, f.log)) != test.attempts {
				t.Fatalf("retained bounded attempts: %v %+v", err, report)
			}
			if test.attempts == 2 && report.Attempts[0].Status != "completed" {
				t.Fatal("first arm lost after second-arm failure")
			}
			last := report.Attempts[len(report.Attempts)-1]
			if test.mode == "malformed" && (last.Transport == nil || len(last.Transport.UsageEvents) != 1 ||
				last.Transport.UsageComplete || last.Metrics["credit_estimate"].Value != nil) {
				t.Fatal("raw partial usage lost or presented as full cost")
			}
			if (test.mode == "mutation" || test.mode == "basis-unavailable") && last.BasisError == nil {
				t.Fatal("post-failure repository basis failure not retained")
			}
			if test.mode == "basis-unavailable" && last.After != nil {
				t.Fatal("unavailable post basis presented as observed")
			}
			summary := f.summary(t)
			if len(summary.Attempts) != test.attempts || len(summary.Groups) != 1 || summary.Groups[0].Verdict.Value != nil {
				t.Fatalf("failed/wrong/unknown task hidden: %+v", summary)
			}
			if test.mode == "wrong-answer" && summary.Groups[0].Metrics["credit_estimate"].KnownPairs != 1 {
				t.Fatal("cheaper wrong attempt excluded from cost inventory")
			}
		})
	}
}

func TestTaskEvaluationShippedVerificationAndAssessment(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	runner := buildTaskExecutable(t, "./cmd/repocue-codex-runner")
	provider := buildTaskExecutable(t, "./internal/app/testdata/task-context/agent.go")
	for _, mode := range []string{"", "final-only", "wrong-command", "partial-output"} {
		t.Run("verification/"+mode, func(t *testing.T) {
			f := newTaskEvaluationFixture(t, binary, runner, provider, "verification")
			t.Setenv("REPOCUE_TEST_MODE", mode)
			report, err := f.run(t)
			if err != nil {
				t.Fatal(err)
			}
			summary := f.summary(t)
			correct := summary.Groups[0].Verdict.Value
			if (correct != nil) != (mode == "") {
				t.Fatalf("check completion awarded from unsupported evidence: %+v", summary)
			}
			if mode == "" {
				for _, attempt := range report.Attempts {
					found := false
					for _, command := range attempt.Transport.Commands {
						if command.Command == "go test ./..." && command.ExitCode != nil && *command.ExitCode == 0 &&
							command.Output != nil && strings.Contains(*command.Output, "ok") && command.OutputComplete {
							found = true
						}
					}
					if !found {
						t.Fatal("actual declared Go test command was not observed")
					}
				}
			}
		})
	}
	t.Run("explanation", func(t *testing.T) {
		f := newTaskEvaluationFixture(t, binary, runner, provider, "explanation")
		report, err := f.run(t)
		if err != nil {
			t.Fatal(err)
		}
		if f.summary(t).Groups[0].Verdict.Value != nil {
			t.Fatal("explanation awarded without independent assessment")
		}
		sources, err := taskbenchmark.Sources(*report.Expected)
		if err != nil {
			t.Fatal(err)
		}
		refs := []taskcontext.Reference{}
		for _, source := range sources {
			refs = append(refs, taskcontext.Reference{Path: source.Path, ContentDigest: source.ContentDigest, Lines: source.Lines})
		}
		assessments := []taskbenchmark.Assessment{}
		for _, attempt := range report.Attempts {
			encoded, _ := json.Marshal(attempt.Transport.FinalResponse)
			assessments = append(assessments, taskbenchmark.Assessment{TaskID: f.spec.ID,
				PolicyDigest: report.PolicyDigest, AttemptID: attempt.ID, AnswerDigest: taskBytesDigest(encoded),
				BasisFingerprint: report.Basis.Fingerprint, Assessor: "local-fixture-only", Rubric: f.spec.Policy.Rubric,
				SourceRefs: refs, Correct: true})
		}
		name := filepath.Join(f.external, "assessments.json")
		taskWriteJSON(t, name, map[string]any{"schema_version": taskbenchmark.EvaluationVersion, "kind": "assessments", "assessments": assessments})
		if f.summary(t, "--assessment-file", name).Groups[0].Verdict.Value == nil {
			t.Fatal("bound fixture assessment unavailable")
		}
		assessments[0].AnswerDigest = taskBytesDigest([]byte("wrong answer"))
		taskWriteJSON(t, name, map[string]any{"schema_version": taskbenchmark.EvaluationVersion, "kind": "assessments", "assessments": assessments})
		stdout, _, err := f.raw("evaluate-task", "summary", "--input-directory", f.external, "--assessment-file", name)
		if err == nil || len(stdout) != 0 {
			t.Fatal("unbound assessment accepted")
		}
	})
}

func TestTaskEvaluationShippedCancellation(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	runner := buildTaskExecutable(t, "./cmd/repocue-codex-runner")
	provider := buildTaskExecutable(t, "./internal/app/testdata/task-context/agent.go")
	f := newTaskEvaluationFixture(t, binary, runner, provider, "discovery")
	t.Setenv("REPOCUE_TEST_MODE", "cancel")
	t.Setenv("REPOCUE_TEST_FAIL_ARM", "task")
	command := exec.Command(binary, f.evalArgs(true)...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if command.ProcessState == nil {
			command.Process.Kill()
			command.Wait()
		}
	}()
	deadline := time.Now().Add(15 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		names, _ := filepath.Glob(filepath.Join(f.external, "task-pair-*", "attempt-task-*", "outer-ready"))
		if len(names) == 1 {
			ready = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("real provider boundary did not reach cancellation point")
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err == nil {
		t.Fatal("cancelled CLI reported success")
	}
	var report evaluation.TaskReport
	if err := taskcontext.DecodeStrict(stdout.Bytes(), "task-evaluation-v1.schema.json#/$defs/report", &report); err != nil {
		t.Fatalf("cancellation report: %v: %s %s", err, stdout.Bytes(), stderr.Bytes())
	}
	if len(report.Attempts) != 2 || report.Attempts[0].Status != "completed" || report.Attempts[1].Status != "cancelled" {
		t.Fatalf("cancelled second arm erased first arm: %+v", report)
	}
	last := report.Attempts[1]
	if last.Transport == nil || len(last.Transport.UsageEvents) != 1 || last.Transport.UsageComplete || last.After == nil {
		t.Fatal("cancelled raw usage or actual post basis lost")
	}
	if len(f.summary(t).Attempts) != 2 || len(taskExecutions(t, f.log)) != 2 {
		t.Fatal("cancellation retried or hidden")
	}
}
