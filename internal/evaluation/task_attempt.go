package evaluation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

const (
	taskEvidenceTimeout = 30 * time.Second
	taskRunnerWaitDelay = 5 * time.Second
	maxTaskRecordBytes  = 16 * 1024 * 1024
)

var errTaskReasoningBounds = errors.New("reasoning output exceeds total output")

type TaskAttempt struct {
	SchemaVersion string                      `json:"schema_version"`
	Kind          string                      `json:"kind"`
	ID            string                      `json:"id"`
	Arm           string                      `json:"arm"`
	Status        string                      `json:"status"`
	StartedAt     time.Time                   `json:"started_at"`
	FinishedAt    *time.Time                  `json:"finished_at"`
	ExitCode      *int                        `json:"exit_code"`
	Before        *repository.ContentEvidence `json:"before"`
	After         *repository.ContentEvidence `json:"after"`
	BasisError    *string                     `json:"basis_error"`
	Transport     *taskbenchmark.Transport    `json:"transport"`
	Artifacts     map[string]string           `json:"artifacts"`
	Delivery      taskbenchmark.Delivery      `json:"delivery"`
	Correctness   taskbenchmark.Outcome       `json:"correctness"`
	Compliance    taskbenchmark.Outcome       `json:"compliance"`
	Metrics       map[string]TaskMetric       `json:"metrics"`
	Eligible      bool                        `json:"eligible"`
	Error         *string                     `json:"error"`
}

func taskDigest(value []byte) string { return taskcontext.Digest(value) }

func writeTaskFile(directory, name string, value []byte) error {
	file, err := os.OpenFile(filepath.Join(directory, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(value)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func writeTaskJSON(directory, name string, value any) error {
	serialized, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".record-")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err := file.Write(append(serialized, '\n')); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, filepath.Join(directory, name))
}

func readTaskFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	value, err := io.ReadAll(io.LimitReader(file, maxTaskRecordBytes+1))
	if err != nil {
		return nil, err
	}
	if len(value) > maxTaskRecordBytes {
		return nil, errors.New("task record exceeds 16 MiB decoding limit; raw artifact retained")
	}
	return value, nil
}

func captureTaskBasis(repo *repository.Repository) (*repository.ContentEvidence, error) {
	ctx, cancel := context.WithTimeout(context.Background(), taskEvidenceTimeout)
	defer cancel()
	capture, err := repo.CaptureEvidence(ctx)
	if err != nil {
		return nil, err
	}
	return &capture.Evidence, nil
}

func runTaskAttempt(ctx context.Context, config TaskConfig, report TaskReport, repo *repository.Repository, arm string) (attempt TaskAttempt, runErr error) {
	directory, err := os.MkdirTemp(report.Directory, "attempt-"+arm+"-")
	if err != nil {
		return attempt, err
	}
	attempt = TaskAttempt{SchemaVersion: taskbenchmark.EvaluationVersion, Kind: "attempt",
		ID: filepath.Base(directory), Arm: arm, Status: "started", StartedAt: time.Now().UTC(),
		Artifacts: map[string]string{}, Delivery: taskbenchmark.Delivery{Status: "not_observed", Provided: []taskbenchmark.Provision{}},
		Correctness: taskbenchmark.Unknown("attempt incomplete"), Compliance: taskbenchmark.Unknown("assessment unavailable"),
		Metrics: taskMetrics(nil, report.Rates, *report.Settings, time.Now().UTC())}
	if err := writeTaskJSON(directory, "attempt-start.json", attempt); err != nil {
		return attempt, err
	}
	defer func() {
		finished := time.Now().UTC()
		attempt.FinishedAt = &finished
		after, basisErr := captureTaskBasis(repo)
		attempt.After = after
		if basisErr != nil {
			message := basisErr.Error()
			attempt.BasisError = &message
		}
		if basisErr == nil && attempt.Before != nil && after.Fingerprint != attempt.Before.Fingerprint {
			message := "repository content changed during task attempt"
			attempt.BasisError = &message
		}
		if attempt.BasisError != nil && runErr == nil {
			runErr = errors.New(*attempt.BasisError)
		}
		for _, name := range []string{"cue.json", "prompt.txt", "request.json", "runner.stdout", "runner.stderr",
			"submitted-prompt.txt", "answer.schema.json", "invocation.json", "provider.jsonl", "provider.stderr"} {
			path := filepath.Join(directory, name)
			file, err := os.Open(path)
			if err == nil {
				hash, err := hashTaskArtifact(file)
				file.Close()
				if err == nil {
					attempt.Artifacts[name] = hash
				}
			}
		}
		if attempt.Transport != nil {
			attempt.Metrics = taskMetrics(attempt.Transport, report.Rates, *report.Settings, attempt.StartedAt)
		}
		addTaskCueMetrics(attempt.Metrics, report, arm)
		if runErr != nil {
			message := runErr.Error()
			attempt.Error = &message
			attempt.Status = "failed"
			if ctx.Err() != nil {
				attempt.Status = "cancelled"
			}
			if attempt.Transport != nil && attempt.Transport.Status == "invalid" {
				attempt.Status = "invalid"
			}
		}
		attempt.Eligible = runErr == nil && attempt.Status == "completed" && attempt.Before != nil &&
			attempt.After != nil && attempt.Before.Fingerprint == report.Basis.Fingerprint &&
			attempt.Before.Fingerprint == attempt.After.Fingerprint
		if err := writeTaskJSON(directory, "attempt.json", attempt); err != nil {
			runErr = fmt.Errorf("retain attempt final record: %w", err)
		}
	}()
	before, err := captureTaskBasis(repo)
	attempt.Before = before
	if err != nil {
		return attempt, err
	}
	if before.Fingerprint != report.Basis.Fingerprint {
		return attempt, errors.New("repository differs before task attempt")
	}
	cueBytes, err := readTaskFile(filepath.Join(report.Directory, arm+".json"))
	if err != nil {
		return attempt, err
	}
	prompt := taskPrompt(report.Task, cueBytes)
	if err := writeTaskFile(directory, "cue.json", cueBytes); err != nil {
		return attempt, err
	}
	if err := writeTaskFile(directory, "prompt.txt", prompt); err != nil {
		return attempt, err
	}
	if report.OutputSchemaDigest == nil {
		return attempt, errors.New("frozen answer schema digest unavailable")
	}
	request := taskbenchmark.RunnerRequest{SchemaVersion: taskbenchmark.RunnerVersion, Kind: "request",
		AttemptID: attempt.ID, Repository: repo.Root, TaskFile: filepath.Join(report.Directory, "benchmark.json"),
		CueFile: filepath.Join(directory, "cue.json"), PromptFile: filepath.Join(directory, "prompt.txt"),
		OutputSchemaFile: filepath.Join(report.Directory, "answer.schema.json"), OutputSchemaDigest: *report.OutputSchemaDigest,
		AttemptDirectory: directory, Settings: *report.Settings}
	if err := writeTaskJSON(directory, "request.json", request); err != nil {
		return attempt, err
	}
	stdout, err := os.OpenFile(filepath.Join(directory, "runner.stdout"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return attempt, err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(filepath.Join(directory, "runner.stderr"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return attempt, err
	}
	defer stderr.Close()
	command := exec.CommandContext(ctx, config.Runner)
	command.Dir = repo.Root
	command.Env = append(os.Environ(), "REPOCUE_EVAL_SCHEMA="+taskbenchmark.RunnerVersion,
		"REPOCUE_TASK_RUNNER_CONFIG="+filepath.Join(directory, "request.json"))
	command.Stdout = stdout
	command.Stderr = stderr
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
	command.WaitDelay = taskRunnerWaitDelay
	execErr := command.Run()
	if command.ProcessState != nil {
		code := command.ProcessState.ExitCode()
		attempt.ExitCode = &code
	}
	if err := stdout.Close(); err != nil {
		return attempt, err
	}
	if err := stderr.Close(); err != nil {
		return attempt, err
	}
	observation, err := readTaskFile(filepath.Join(directory, "runner.stdout"))
	if err != nil {
		return attempt, err
	}
	var transport taskbenchmark.Transport
	if err := taskcontext.DecodeStrict(observation, "task-runner-v1.schema.json#/$defs/observation", &transport); err != nil {
		return attempt, fmt.Errorf("task runner observation: %w", err)
	}
	attempt.Transport = &transport
	if transport.AttemptID != attempt.ID {
		return attempt, errors.New("runner attempt identity mismatch")
	}
	if execErr != nil {
		return attempt, fmt.Errorf("task runner failed: %w", execErr)
	}
	if err := verifyTaskSchema(directory, report, transport); err != nil {
		return attempt, err
	}
	submitted, err := readTaskFile(filepath.Join(directory, "submitted-prompt.txt"))
	if err != nil {
		return attempt, err
	}
	if transport.PromptDigest == nil || transport.CueDigest == nil ||
		*transport.PromptDigest != taskDigest(submitted) || *transport.CueDigest != taskDigest(cueBytes) ||
		!bytes.Equal(submitted, prompt) || !bytes.Contains(submitted, cueBytes) {
		return attempt, errors.New("actual submitted content does not match the frozen task cue/prompt")
	}
	delivery, err := taskbenchmark.DeliveryEvidence(*report.Expected, cueBytes, submitted, transport)
	if err != nil {
		return attempt, err
	}
	attempt.Delivery = delivery
	correct, compliance, err := taskbenchmark.Score(report.Task, *report.Expected, transport, nil, before.Fingerprint)
	if err != nil {
		return attempt, err
	}
	attempt.Correctness = correct
	attempt.Compliance = compliance
	attempt.Status = "completed"
	return attempt, nil
}

func taskPrompt(spec taskbenchmark.Spec, cueBytes []byte) []byte {
	targets, _ := json.Marshal(spec.Targets)
	var value bytes.Buffer
	fmt.Fprintf(&value, "%s\n\nTask ID: %s\nTask kind: %s\nContext declaration: %s\nTargets: %s\n",
		spec.Prompt, spec.ID, spec.TaskKind, spec.ContextFile, targets)
	if spec.Policy != nil {
		for _, check := range spec.Policy.Checks {
			fmt.Fprintf(&value, "Required check %s: %s\n", check.ID, check.Command)
		}
	}
	value.WriteString("\nUse the supplied context first. Inspect original sources when needed. Report uncertainty. Return only a task-answer-1 JSON answer.\n\nRepoCue context:\n")
	value.Write(cueBytes)
	value.WriteByte('\n')
	return value.Bytes()
}

func hashTaskArtifact(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}
