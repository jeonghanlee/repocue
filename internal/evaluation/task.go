package evaluation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/docs/schema"
	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

type TaskConfig struct {
	Repository      string
	CacheDir        string
	TaskFile        string
	OutputDirectory string
	CLI             string
	Runner          string
	Order           string
	Settings        taskbenchmark.Settings
	RatesFile       string
}

type TaskProcessCost struct {
	Operation       string  `json:"operation"`
	DurationMS      float64 `json:"duration_ms"`
	OutputBytes     int64   `json:"output_bytes"`
	EstimatedTokens int64   `json:"estimated_tokens"`
	OutputDigest    string  `json:"output_digest"`
	ExitCode        *int    `json:"exit_code"`
	Error           *string `json:"error"`
}

type TaskReport struct {
	SchemaVersion      string                      `json:"schema_version"`
	Kind               string                      `json:"kind"`
	ID                 string                      `json:"id"`
	Directory          string                      `json:"directory"`
	CreatedAt          time.Time                   `json:"created_at"`
	Status             string                      `json:"status"`
	Task               taskbenchmark.Spec          `json:"task"`
	TaskDigest         string                      `json:"task_digest"`
	OutputSchemaDigest *string                     `json:"output_schema_digest"`
	PolicyDigest       string                      `json:"policy_digest"`
	Rates              *taskbenchmark.Rates        `json:"rates"`
	RateDigest         string                      `json:"rate_digest"`
	Settings           *taskbenchmark.Settings     `json:"settings"`
	Basis              *repository.ContentEvidence `json:"basis"`
	Expected           *cue.TaskEnvelope           `json:"expected"`
	Order              []string                    `json:"order"`
	Maintenance        []TaskProcessCost           `json:"maintenance"`
	Baseline           TaskMetric                  `json:"baseline"`
	CueCosts           []TaskProcessCost           `json:"cue_costs"`
	Attempts           []TaskAttempt               `json:"attempts"`
	Error              *string                     `json:"error"`
}

// RunTask prepares actual CLI context without invoking an agent by default.
// Runner mode performs one explicitly ordered pair and retains every failure.
func RunTask(ctx context.Context, config TaskConfig) (report TaskReport, runErr error) {
	taskBytes, err := readTaskFile(config.TaskFile)
	if err != nil {
		return report, err
	}
	spec, err := taskbenchmark.Decode(taskBytes)
	if err != nil {
		return report, err
	}
	if config.Order == "" {
		config.Order = "overview,task"
	}
	order := strings.Split(config.Order, ",")
	if len(order) != 2 || !((order[0] == "overview" && order[1] == "task") || (order[0] == "task" && order[1] == "overview")) {
		return report, errors.New("order must be overview,task or task,overview")
	}
	if config.Runner != "" {
		serialized, _ := json.Marshal(config.Settings)
		if err := taskcontext.DecodeStrict(serialized, "task-benchmark-v1.schema.json#/$defs/settings", &config.Settings); err != nil {
			return report, err
		}
		if _, err := resolveExecutable(config.Runner); err != nil {
			return report, err
		}
	}
	repo, err := repository.Open(ctx, config.Repository)
	if err != nil {
		return report, err
	}
	output, err := taskOutputRoot(repo.Root, config.OutputDirectory)
	if err != nil {
		return report, err
	}
	if err := os.MkdirAll(output, 0o700); err != nil {
		return report, err
	}
	directory, err := os.MkdirTemp(output, "task-pair-")
	if err != nil {
		return report, err
	}
	report = TaskReport{SchemaVersion: taskbenchmark.EvaluationVersion, Kind: "preparation",
		ID: filepath.Base(directory), Directory: directory, CreatedAt: time.Now().UTC(), Status: "preparing",
		Task: spec, TaskDigest: taskDigest(taskBytes), PolicyDigest: taskbenchmark.PolicyDigest(spec),
		Order: order, Maintenance: []TaskProcessCost{}, CueCosts: []TaskProcessCost{}, Attempts: []TaskAttempt{},
		Baseline: taskUnknown("ms", "existing maintained cache; initial baseline cost not measured here")}
	if config.Runner != "" {
		report.Settings = &config.Settings
	}
	defer func() {
		if runErr != nil {
			message := runErr.Error()
			report.Error = &message
			report.Status = "failed"
		}
		if err := writeTaskJSON(directory, "manifest.json", report); err != nil {
			runErr = fmt.Errorf("retain task manifest: %w", err)
		}
	}()
	if err := writeTaskFile(directory, "benchmark.json", taskBytes); err != nil {
		return report, err
	}
	outputSchema, err := taskbenchmark.OutputSchema()
	if err != nil {
		return report, err
	}
	if err := writeTaskFile(directory, "answer.schema.json", outputSchema); err != nil {
		return report, err
	}
	outputSchemaDigest := taskDigest(outputSchema)
	report.OutputSchemaDigest = &outputSchemaDigest
	if config.RatesFile != "" {
		serialized, err := readTaskFile(config.RatesFile)
		if err != nil {
			return report, err
		}
		var rates taskbenchmark.Rates
		if err := taskcontext.DecodeStrict(serialized, "task-benchmark-v1.schema.json#/$defs/rates", &rates); err != nil {
			return report, err
		}
		if err := taskbenchmark.ValidateRates(rates, report.CreatedAt); err != nil {
			return report, err
		}
		report.Rates = &rates
		report.RateDigest = taskRateDigest(&rates)
		if err := writeTaskFile(directory, "rates.json", serialized); err != nil {
			return report, err
		}
	}
	if err := writeTaskJSON(directory, "manifest.json", report); err != nil {
		return report, err
	}
	common := []string{"--repository", repo.Root, "--cache-dir", config.CacheDir}
	refresh, _, err := taskCLI(ctx, config.CLI, directory, "refresh", append([]string{"refresh"}, common...))
	report.Maintenance = append(report.Maintenance, refresh)
	if err != nil {
		return report, err
	}
	before, err := captureTaskBasis(repo)
	if err != nil {
		return report, err
	}
	report.Basis = before
	for _, arm := range []string{"overview", "task"} {
		args := append([]string{"cue"}, common...)
		args = append(args, "--view", arm, "--max-tokens", fmt.Sprint(spec.MaxTokens))
		if arm == "task" {
			args = append(args, "--context-file", spec.ContextFile, "--task-kind", spec.TaskKind)
			for _, target := range spec.Targets {
				args = append(args, "--target", target)
			}
		}
		cost, value, err := taskCLI(ctx, config.CLI, directory, arm, args)
		report.CueCosts = append(report.CueCosts, cost)
		if err != nil {
			return report, err
		}
		if arm == "task" {
			expected, err := cue.DecodeTask(value)
			if err != nil {
				return report, err
			}
			if expected.Basis.RepositoryDigest != before.TrackedDigest || expected.Basis.StatusDigest != before.StatusDigest {
				return report, errors.New("task cue does not match captured repository basis")
			}
			if _, err := taskbenchmark.Sources(expected); err != nil {
				return report, err
			}
			report.Expected = &expected
		} else {
			if err := schema.Validate("cue-v1.schema.json", bytes.TrimSpace(value)); err != nil {
				return report, err
			}
		}
	}
	after, err := captureTaskBasis(repo)
	if err != nil || after.Fingerprint != before.Fingerprint {
		return report, errors.New("repository changed during cue preparation")
	}
	report.Status = "prepared"
	if config.Runner == "" {
		return report, nil
	}
	report.Kind = "pair"
	report.Status = "running"
	if err := writeTaskJSON(directory, "manifest.json", report); err != nil {
		return report, err
	}
	for _, arm := range order {
		attempt, err := runTaskAttempt(ctx, config, report, repo, arm)
		if attempt.ID != "" {
			report.Attempts = append(report.Attempts, attempt)
		}
		if err != nil {
			return report, err
		}
		if err := writeTaskJSON(directory, "manifest.json", report); err != nil {
			return report, err
		}
	}
	if len(report.Attempts) == 2 && report.Attempts[0].Transport.CodexVersion != report.Attempts[1].Transport.CodexVersion {
		return report, errors.New("provider version changed between paired attempts")
	}
	report.Status = "completed"
	return report, nil
}

func taskCLI(ctx context.Context, binary, directory, operation string, args []string) (TaskProcessCost, []byte, error) {
	cost := TaskProcessCost{Operation: operation}
	stdout, err := os.OpenFile(filepath.Join(directory, operation+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return cost, nil, err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(filepath.Join(directory, operation+".stderr"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return cost, nil, err
	}
	defer stderr.Close()
	command := exec.CommandContext(ctx, binary, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	started := time.Now()
	execErr := command.Run()
	cost.DurationMS = float64(time.Since(started)) / float64(time.Millisecond)
	if command.ProcessState != nil {
		exit := command.ProcessState.ExitCode()
		cost.ExitCode = &exit
	}
	if info, err := stdout.Stat(); err == nil {
		cost.OutputBytes = info.Size()
		cost.EstimatedTokens = info.Size() / 4
		if info.Size()%4 != 0 {
			cost.EstimatedTokens++
		}
	}
	if err := stdout.Close(); err != nil {
		return cost, nil, err
	}
	if err := stderr.Close(); err != nil {
		return cost, nil, err
	}
	file, err := os.Open(filepath.Join(directory, operation+".json"))
	if err == nil {
		cost.OutputDigest, err = hashTaskArtifact(file)
		file.Close()
	}
	if err != nil {
		return cost, nil, err
	}
	if execErr != nil {
		message := execErr.Error()
		cost.Error = &message
		return cost, nil, fmt.Errorf("%s CLI failed; raw output retained: %w", operation, execErr)
	}
	value, err := readTaskFile(filepath.Join(directory, operation+".json"))
	return cost, value, err
}

func taskOutputRoot(repositoryRoot, name string) (string, error) {
	if name == "" {
		return "", errors.New("external output-directory is required")
	}
	absolute, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	parent := absolute
	missing := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(parent)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			relative, err := filepath.Rel(repositoryRoot, resolved)
			if err != nil || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
				return "", errors.New("task evidence must be outside the evaluated repository")
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) || filepath.Dir(parent) == parent {
			return "", err
		}
		missing = append(missing, filepath.Base(parent))
		parent = filepath.Dir(parent)
	}
}

type TaskInventory struct {
	PairID      string                 `json:"pair_id"`
	AttemptID   string                 `json:"attempt_id"`
	Arm         string                 `json:"arm"`
	Status      string                 `json:"status"`
	Eligible    bool                   `json:"eligible"`
	Delivery    taskbenchmark.Delivery `json:"delivery"`
	Correctness taskbenchmark.Outcome  `json:"correctness"`
	Compliance  taskbenchmark.Outcome  `json:"compliance"`
	Metrics     map[string]TaskMetric  `json:"metrics"`
	Error       *string                `json:"error"`
}

type TaskCoverage struct {
	KnownPairs       int      `json:"known_pairs"`
	UnknownPairs     int      `json:"unknown_pairs"`
	MedianDifference *float64 `json:"median_difference"`
	MinDifference    *float64 `json:"min_difference"`
	MaxDifference    *float64 `json:"max_difference"`
}

type TaskGroup struct {
	TaskID           string                  `json:"task_id"`
	TaskDigest       string                  `json:"task_digest"`
	PolicyDigest     string                  `json:"policy_digest"`
	BasisFingerprint string                  `json:"basis_fingerprint"`
	PairCount        int                     `json:"pair_count"`
	OverviewFirst    int                     `json:"overview_first"`
	TaskFirst        int                     `json:"task_first"`
	Metrics          map[string]TaskCoverage `json:"metrics"`
	Verdict          taskbenchmark.Outcome   `json:"verdict"`
}

type TaskSummary struct {
	SchemaVersion string          `json:"schema_version"`
	Kind          string          `json:"kind"`
	Attempts      []TaskInventory `json:"attempts"`
	Groups        []TaskGroup     `json:"groups"`
	Errors        []string        `json:"errors"`
}

type taskSummaryPair struct {
	report   TaskReport
	attempts map[string]TaskAttempt
}

// SummarizeTasks reads retained attempts, including incomplete journals, without
// launching a process. Pair coverage and task success gate every verdict.
func SummarizeTasks(directory, assessmentFile string) (TaskSummary, error) {
	result := TaskSummary{SchemaVersion: taskbenchmark.EvaluationVersion, Kind: "summary",
		Attempts: []TaskInventory{}, Groups: []TaskGroup{}, Errors: []string{}}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return result, err
	}
	directory, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return result, err
	}
	assessments := map[string]taskbenchmark.Assessment{}
	if assessmentFile != "" {
		serialized, err := readTaskFile(assessmentFile)
		if err != nil {
			return result, err
		}
		var bundle struct {
			SchemaVersion string                     `json:"schema_version"`
			Kind          string                     `json:"kind"`
			Assessments   []taskbenchmark.Assessment `json:"assessments"`
		}
		if err := taskcontext.DecodeStrict(serialized, "task-evaluation-v1.schema.json#/$defs/assessments", &bundle); err != nil {
			return result, err
		}
		for _, assessment := range bundle.Assessments {
			if _, found := assessments[assessment.AttemptID]; found {
				return result, errors.New("duplicate attempt assessment")
			}
			assessments[assessment.AttemptID] = assessment
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return result, err
	}
	groups := map[string][]taskSummaryPair{}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "task-pair-") {
			continue
		}
		root := filepath.Join(directory, entry.Name())
		serialized, err := readTaskFile(filepath.Join(root, "manifest.json"))
		if err != nil {
			result.Errors = append(result.Errors, entry.Name()+": "+err.Error())
			appendInvalidTaskAttempts(&result, root, entry.Name(), err.Error())
			continue
		}
		var report TaskReport
		if err := taskcontext.DecodeStrict(serialized, "task-evaluation-v1.schema.json#/$defs/report", &report); err != nil {
			result.Errors = append(result.Errors, entry.Name()+": "+err.Error())
			appendInvalidTaskAttempts(&result, root, entry.Name(), err.Error())
			continue
		}
		if report.Kind != "pair" {
			continue
		}
		if report.ID != entry.Name() || report.Directory != root || report.Basis == nil || report.Expected == nil || report.Settings == nil {
			result.Errors = append(result.Errors, entry.Name()+": incomplete pair manifest")
			appendInvalidTaskAttempts(&result, root, entry.Name(), "incomplete pair manifest")
			continue
		}
		keyBytes, _ := json.Marshal([]any{report.TaskDigest, report.PolicyDigest, report.OutputSchemaDigest, report.Basis.Fingerprint, report.Settings, report.RateDigest})
		key := taskDigest(keyBytes)
		armAttempts := map[string]TaskAttempt{}
		children, err := os.ReadDir(root)
		if err != nil {
			return result, err
		}
		input, err := readTaskFile(filepath.Join(root, "benchmark.json"))
		basisValid := err == nil && taskDigest(input) == report.TaskDigest && verifyTaskPair(root, report) == nil
		for _, child := range children {
			if !child.IsDir() || !strings.HasPrefix(child.Name(), "attempt-") {
				continue
			}
			attemptRoot := filepath.Join(root, child.Name())
			value, err := readTaskFile(filepath.Join(attemptRoot, "attempt.json"))
			if os.IsNotExist(err) {
				value, err = readTaskFile(filepath.Join(attemptRoot, "attempt-start.json"))
			}
			if err != nil {
				result.Errors = append(result.Errors, child.Name()+": "+err.Error())
				result.Attempts = append(result.Attempts, invalidTaskInventory(report.ID, child.Name(), err.Error()))
				continue
			}
			var attempt TaskAttempt
			if err := taskcontext.DecodeStrict(value, "task-evaluation-v1.schema.json#/$defs/attempt", &attempt); err != nil {
				result.Errors = append(result.Errors, child.Name()+": "+err.Error())
				result.Attempts = append(result.Attempts, invalidTaskInventory(report.ID, child.Name(), err.Error()))
				continue
			}
			if attempt.ID != child.Name() || (attempt.Arm != "overview" && attempt.Arm != "task") {
				result.Errors = append(result.Errors, child.Name()+": attempt identity mismatch")
				result.Attempts = append(result.Attempts, invalidTaskInventory(report.ID, child.Name(), "attempt identity mismatch"))
				continue
			}
			valid := basisValid && verifyTaskAttempt(attemptRoot, report, &attempt) == nil
			if !valid {
				attempt.Eligible = false
				message := "retained task/artifact digest mismatch"
				attempt.Error = &message
			}
			if attempt.Transport != nil {
				attempt.Metrics = taskMetrics(attempt.Transport, report.Rates, *report.Settings, attempt.StartedAt)
				var assessment *taskbenchmark.Assessment
				if found, ok := assessments[attempt.ID]; ok {
					assessment = &found
					delete(assessments, attempt.ID)
				}
				correct, compliance, err := taskbenchmark.Score(report.Task, *report.Expected, *attempt.Transport, assessment, report.Basis.Fingerprint)
				if err != nil {
					return result, err
				}
				attempt.Correctness = correct
				attempt.Compliance = compliance
			}
			if !valid {
				attempt.Metrics = taskMetrics(nil, report.Rates, *report.Settings, attempt.StartedAt)
				attempt.Correctness = taskbenchmark.Unknown("artifact integrity unavailable")
				attempt.Delivery = taskbenchmark.Delivery{Status: "not_observed", Provided: []taskbenchmark.Provision{}}
			} else {
				addTaskCueMetrics(attempt.Metrics, report, attempt.Arm)
			}
			if _, exists := armAttempts[attempt.Arm]; exists {
				return result, errors.New("pair contains repeated arm; no retry selection is allowed")
			}
			armAttempts[attempt.Arm] = attempt
			result.Attempts = append(result.Attempts, TaskInventory{PairID: report.ID, AttemptID: attempt.ID, Arm: attempt.Arm,
				Status: attempt.Status, Eligible: attempt.Eligible, Delivery: attempt.Delivery, Correctness: attempt.Correctness,
				Compliance: attempt.Compliance, Metrics: attempt.Metrics, Error: attempt.Error})
		}
		groups[key] = append(groups[key], taskSummaryPair{report: report, attempts: armAttempts})
	}
	if len(assessments) > 0 {
		return result, errors.New("assessment names an unknown retained attempt")
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		pairs := groups[key]
		first := pairs[0].report
		group := TaskGroup{TaskID: first.Task.ID, TaskDigest: first.TaskDigest, PolicyDigest: first.PolicyDigest,
			BasisFingerprint: first.Basis.Fingerprint, PairCount: len(pairs), Metrics: map[string]TaskCoverage{},
			Verdict: taskbenchmark.Unknown("frozen policy, successful tasks and complete required pair/evidence coverage are required")}
		good := first.Task.Policy != nil && len(pairs) == first.Task.Policy.RequiredPairs
		version := ""
		for _, pair := range pairs {
			for _, arm := range []string{"overview", "task"} {
				attempt := pair.attempts[arm]
				if version == "" && attempt.Transport != nil {
					version = attempt.Transport.CodexVersion
				}
			}
		}
		values := map[string][]float64{}
		for _, pair := range pairs {
			if len(pair.report.Order) > 0 && pair.report.Order[0] == "overview" {
				group.OverviewFirst++
			} else {
				group.TaskFirst++
			}
			a, aOK := pair.attempts["overview"]
			b, bOK := pair.attempts["task"]
			comparable := pair.report.Status == "completed" && pair.report.Error == nil && aOK && bOK && a.Eligible && b.Eligible && a.Transport != nil && b.Transport != nil &&
				a.Transport.CodexVersion == b.Transport.CodexVersion && a.Transport.CodexVersion == version
			if !comparable || !a.Delivery.Complete || !b.Delivery.Complete || !outcomeTrue(a.Correctness) || !outcomeTrue(b.Correctness) {
				good = false
			}
			for _, name := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens",
				"total_tokens", "credit_estimate", "agent_duration_ms", "command_calls", "cpu_ms", "memory_bytes",
				"filesystem_io_bytes", "task_ready_ms", "account_billing_credits", "cue_duration_ms", "cue_bytes", "cue_estimated_tokens"} {
				coverage := group.Metrics[name]
				left, right := a.Metrics[name], b.Metrics[name]
				if comparable && left.Value != nil && right.Value != nil {
					coverage.KnownPairs++
					values[name] = append(values[name], *left.Value-*right.Value)
				} else {
					coverage.UnknownPairs++
				}
				group.Metrics[name] = coverage
			}
		}
		for name, coverage := range group.Metrics {
			samples := values[name]
			if len(samples) > 0 {
				sort.Float64s(samples)
				minimum, maximum := samples[0], samples[len(samples)-1]
				median := taskMedian(samples)
				coverage.MedianDifference = &median
				coverage.MinDifference = &minimum
				coverage.MaxDifference = &maximum
				group.Metrics[name] = coverage
			}
		}
		credit := group.Metrics["credit_estimate"]
		if good && credit.UnknownPairs == 0 && credit.MedianDifference != nil {
			group.Verdict = taskbenchmark.Result(*credit.MedianDifference > 0, "complete successful pairs; descriptive overview-minus-task median credit estimate")
		}
		if len(result.Errors) > 0 {
			group.Verdict = taskbenchmark.Unknown("retained evidence contains unreadable or invalid records")
		}
		result.Groups = append(result.Groups, group)
	}
	return result, nil
}

func outcomeTrue(outcome taskbenchmark.Outcome) bool { return outcome.Value != nil && *outcome.Value }
func invalidTaskInventory(pair, attempt, message string) TaskInventory {
	return TaskInventory{PairID: pair, AttemptID: attempt, Arm: "unknown", Status: "invalid", Error: &message,
		Delivery:    taskbenchmark.Delivery{Status: "not_observed", Provided: []taskbenchmark.Provision{}},
		Correctness: taskbenchmark.Unknown("retained attempt unreadable"),
		Compliance:  taskbenchmark.Unknown("retained attempt unreadable"),
		Metrics:     taskMetrics(nil, nil, taskbenchmark.Settings{}, time.Time{})}
}

func appendInvalidTaskAttempts(summary *TaskSummary, root, pair, message string) {
	children, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, child := range children {
		if child.IsDir() && strings.HasPrefix(child.Name(), "attempt-") {
			summary.Attempts = append(summary.Attempts, invalidTaskInventory(pair, child.Name(), message))
		}
	}
}
func taskMedian(values []float64) float64 {
	middle := len(values) / 2
	if len(values)%2 != 0 {
		return values[middle]
	}
	return values[middle-1]/2 + values[middle]/2
}
