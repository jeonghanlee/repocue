package evaluation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jeonghanlee/repocue/internal/claudeadapter"
	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/promptcache"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

type CacheConfig struct {
	Repository, CacheDir, BenchmarkFile, OutputDirectory, CLI string
}
type CacheReport struct {
	SchemaVersion      string                      `json:"schema_version"`
	Kind               string                      `json:"kind"`
	ID                 string                      `json:"id"`
	Directory          string                      `json:"directory"`
	Repository         string                      `json:"repository"`
	CreatedAt          time.Time                   `json:"created_at"`
	Status             string                      `json:"status"`
	BenchmarkDigest    string                      `json:"benchmark_digest"`
	CueDigest          *string                     `json:"cue_digest"`
	AnswerSchemaDigest *string                     `json:"answer_schema_digest"`
	Basis              *repository.ContentEvidence `json:"basis"`
	Spec               promptcache.Spec            `json:"spec"`
	Preparation        []TaskProcessCost           `json:"preparation"`
	Requests           map[string]string           `json:"requests"`
	Error              *string                     `json:"error"`
}
type CacheObservation struct {
	HTTPAttempted  bool                        `json:"http_attempted"`
	RequestWritten bool                        `json:"request_written"`
	SchemaVersion  string                      `json:"schema_version"`
	StepID         string                      `json:"step_id"`
	Endpoint       string                      `json:"endpoint"`
	RequestDigest  string                      `json:"request_digest"`
	StartedAt      time.Time                   `json:"started_at"`
	FinishedAt     *time.Time                  `json:"finished_at"`
	HTTPStartedAt  *time.Time                  `json:"http_started_at"`
	HTTPFinishedAt *time.Time                  `json:"http_finished_at"`
	Before         *repository.ContentEvidence `json:"before"`
	After          *repository.ContentEvidence `json:"after"`
	DurationMS     *float64                    `json:"duration_ms"`
	HTTPStatus     *int                        `json:"http_status"`
	RequestID      *string                     `json:"request_id"`
	ResponseDigest *string                     `json:"response_digest"`
	BodyComplete   bool                        `json:"body_complete"`
	Status         string                      `json:"status"`
	Error          *string                     `json:"error"`
}

// PrepareCache freezes requests through the shipped refresh/task-cue commands.
// It never reads credentials or opens a provider connection.
func PrepareCache(ctx context.Context, config CacheConfig) (report CacheReport, runErr error) {
	raw, err := readTaskFile(config.BenchmarkFile)
	if err != nil {
		return report, err
	}
	spec, err := promptcache.DecodeSpec(raw)
	if err != nil {
		return report, err
	}
	repo, err := repository.Open(ctx, config.Repository)
	if err != nil {
		return report, err
	}
	output, err := taskOutputRoot(repo.Root, config.OutputDirectory)
	if err != nil {
		return report, err
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		return report, err
	}
	directory, err := os.MkdirTemp(output, "cache-eval-")
	if err != nil {
		return report, err
	}
	report = CacheReport{SchemaVersion: promptcache.EvaluationVersion, Kind: "preparation",
		ID: filepath.Base(directory), Directory: directory, Repository: repo.Root, CreatedAt: time.Now().UTC(),
		Status: "preparing", BenchmarkDigest: taskDigest(raw), Spec: spec, Preparation: []TaskProcessCost{}, Requests: map[string]string{}}
	defer func() {
		if runErr != nil {
			message := runErr.Error()
			report.Error = &message
			report.Status = "failed"
		}
		if err := writeTaskJSON(directory, "manifest.json", report); err != nil {
			runErr = err
		}
	}()
	if err := writeTaskFile(directory, "benchmark.json", raw); err != nil {
		return report, err
	}
	answerSchema, err := promptcache.AnswerSchema()
	if err != nil {
		return report, err
	}
	digest := taskDigest(answerSchema)
	report.AnswerSchemaDigest = &digest
	if err := writeTaskFile(directory, "answer.schema.json", answerSchema); err != nil {
		return report, err
	}
	if err := writeTaskJSON(directory, "manifest.json", report); err != nil {
		return report, err
	}
	common := []string{"--repository", repo.Root, "--cache-dir", config.CacheDir}
	cost, _, err := taskCLI(ctx, config.CLI, directory, "refresh", append([]string{"refresh"}, common...))
	report.Preparation = append(report.Preparation, cost)
	if err != nil {
		return report, err
	}
	capture, err := repo.CaptureEvidence(ctx)
	if err != nil {
		return report, err
	}
	report.Basis = &capture.Evidence
	args := append([]string{"cue"}, common...)
	args = append(args, "--view", "task", "--context-file", spec.ContextFile, "--task-kind", spec.TaskKind,
		"--max-tokens", fmt.Sprint(spec.MaxContextTokens))
	for _, target := range spec.Targets {
		args = append(args, "--target", target)
	}
	cost, fullCue, err := taskCLI(ctx, config.CLI, directory, "task", args)
	report.Preparation = append(report.Preparation, cost)
	if err != nil {
		return report, err
	}
	expected, err := cue.DecodeTask(fullCue)
	if err != nil {
		return report, err
	}
	if err := validateCacheSources(ctx, repo, capture, expected); err != nil {
		return report, err
	}
	cueDigest := taskDigest(fullCue)
	report.CueDigest = &cueDigest
	for _, step := range spec.Steps {
		request, err := promptcache.Assemble(spec, step, fullCue, report.BenchmarkDigest, report.Basis.Fingerprint)
		if err != nil {
			return report, err
		}
		value, err := json.Marshal(request)
		if err != nil {
			return report, err
		}
		if err := taskcontext.DecodeStrict(value, "cache-request-v1.schema.json", &request); err != nil {
			return report, err
		}
		if err := writeTaskFile(directory, "request-"+step.ID+".json", value); err != nil {
			return report, err
		}
		report.Requests[step.ID] = taskDigest(value)
	}
	after, err := repo.CaptureEvidence(ctx)
	if err != nil || after.Evidence.Fingerprint != capture.Evidence.Fingerprint {
		return report, errors.New("repository changed during cache preparation")
	}
	report.Status = "prepared"
	return report, nil
}

func canonicalCacheDirectory(name string) (string, error) {
	absolute, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	if resolved != absolute {
		return "", errors.New("linked cache evidence directory is not allowed")
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.IsDir() {
		return "", errors.New("cache evidence directory is not a directory")
	}
	return absolute, nil
}
func readCacheFile(root, name string) ([]byte, error) {
	if filepath.Base(name) != name {
		return nil, errors.New("invalid cache artifact name")
	}
	info, err := os.Lstat(filepath.Join(root, name))
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("cache artifact must be a regular unlinked file")
	}
	return readTaskFile(filepath.Join(root, name))
}
func loadCacheReport(name string) (CacheReport, []byte, error) {
	var report CacheReport
	root, err := canonicalCacheDirectory(name)
	if err != nil {
		return report, nil, err
	}
	raw, err := readCacheFile(root, "manifest.json")
	if err != nil {
		return report, nil, err
	}
	if err := taskcontext.DecodeStrict(raw, "cache-evaluation-v1.schema.json#/$defs/report", &report); err != nil {
		return report, nil, err
	}
	if report.Directory != root || report.ID != filepath.Base(root) || report.Status != "prepared" || report.Error != nil ||
		!validTaskBasis(report.Basis) || report.CueDigest == nil || report.AnswerSchemaDigest == nil {
		return report, nil, errors.New("cache preparation is incomplete or has incompatible directory/basis")
	}
	input, err := readCacheFile(root, "benchmark.json")
	if err != nil || taskDigest(input) != report.BenchmarkDigest {
		return report, nil, errors.New("benchmark digest mismatch")
	}
	spec, err := promptcache.DecodeSpec(input)
	if err != nil || !sameTaskJSON(spec, report.Spec) {
		return report, nil, errors.New("frozen cache benchmark mismatch")
	}
	if spec.Rates != nil && promptcache.ValidateRates(*spec.Rates, spec.Provider, report.CreatedAt) != nil {
		return report, nil, errors.New("rate date incompatible with preparation")
	}
	answer, err := readCacheFile(root, "answer.schema.json")
	if err != nil {
		return report, nil, err
	}
	embedded, err := promptcache.AnswerSchema()
	if err != nil || !bytes.Equal(answer, embedded) || taskDigest(answer) != *report.AnswerSchemaDigest {
		return report, nil, errors.New("answer schema mismatch")
	}
	fullCue, err := readCacheFile(root, "task.json")
	if err != nil || taskDigest(fullCue) != *report.CueDigest {
		return report, nil, errors.New("frozen cue digest mismatch")
	}
	expected, err := cue.DecodeTask(fullCue)
	if err != nil || expected.Basis.RepositoryDigest != report.Basis.TrackedDigest || expected.Basis.StatusDigest != report.Basis.StatusDigest {
		return report, nil, errors.New("frozen cue basis mismatch")
	}
	if len(report.Preparation) != 2 || len(report.Requests) != len(spec.Steps) {
		return report, nil, errors.New("cache preparation inventory incomplete")
	}
	for i, cost := range report.Preparation {
		operation := []string{"refresh", "task"}[i]
		data, err := readCacheFile(root, operation+".json")
		if err != nil || cost.Operation != operation || cost.ExitCode == nil || *cost.ExitCode != 0 || cost.Error != nil ||
			cost.OutputDigest != taskDigest(data) || cost.OutputBytes != int64(len(data)) ||
			cost.EstimatedTokens != int64((len(data)+3)/4) {
			return report, nil, errors.New("preparation process evidence mismatch")
		}
	}
	for _, step := range spec.Steps {
		request, err := promptcache.Assemble(spec, step, fullCue, report.BenchmarkDigest, report.Basis.Fingerprint)
		if err != nil {
			return report, nil, err
		}
		want, err := json.Marshal(request)
		if err != nil {
			return report, nil, err
		}
		got, err := readCacheFile(root, "request-"+step.ID+".json")
		if err != nil || !bytes.Equal(want, got) || taskDigest(got) != report.Requests[step.ID] {
			return report, nil, errors.New("retained request does not reconstruct from original sources")
		}
	}
	return report, fullCue, nil
}
func validateCacheSources(ctx context.Context, repo *repository.Repository, capture repository.EvidenceCapture, expected cue.TaskEnvelope) error {
	if expected.Repository.ID != repo.ID || !sameTaskJSON(expected.Repository.Head, capture.Scan.Basis.Head) ||
		!sameTaskJSON(expected.Repository.Branch, capture.Scan.Basis.Branch) ||
		expected.Basis.RepositoryDigest != capture.Evidence.TrackedDigest || expected.Basis.StatusDigest != capture.Evidence.StatusDigest {
		return errors.New("current repository does not match frozen task cue")
	}
	state := model.CurrentState{Repository: model.Repository{Root: repo.Root}, Files: capture.Scan.Files}
	reader, err := taskcontext.NewReader(state)
	if err != nil {
		return err
	}
	defer reader.Close()
	_, digest, err := reader.Declaration(ctx, expected.Content.Declaration.Path)
	if err != nil || digest != expected.Content.Declaration.ContentDigest {
		return errors.New("current declaration differs from frozen pin")
	}
	for _, source := range expected.Content.Sources {
		actual, err := reader.Read(ctx, taskcontext.Reference{Path: source.Path, ContentDigest: source.ContentDigest, Lines: source.Lines}, maxTaskRecordBytes)
		if err != nil || actual.Overflow || !sameTaskJSON(actual, source) {
			return errors.New("current source differs from frozen original")
		}
	}
	return nil
}

// RunCacheStep reserves exactly one frozen step before checking credentials.
// Failed and interrupted reservations cannot be selected for another run.
func RunCacheStep(ctx context.Context, directory, stepID, endpoint string) (observation CacheObservation, runErr error) {
	report, fullCue, err := loadCacheReport(directory)
	if err != nil {
		return observation, err
	}
	if endpoint == "" {
		endpoint = claudeadapter.DefaultEndpoint
	}
	if err := claudeadapter.ValidateEndpoint(endpoint); err != nil {
		return observation, err
	}
	index := -1
	for i, step := range report.Spec.Steps {
		if step.ID == stepID {
			index = i
			break
		}
	}
	if index < 0 {
		return observation, errors.New("step is not in frozen request schedule")
	}
	for _, previous := range report.Spec.Steps[:index] {
		path := filepath.Join(report.Directory, "attempt-"+previous.ID)
		if _, err := canonicalCacheDirectory(path); err != nil {
			return observation, errors.New("frozen preceding step has not finished")
		}
		raw, err := readCacheFile(path, "observation.json")
		if err != nil {
			return observation, errors.New("frozen preceding step has not finished")
		}
		var prior CacheObservation
		if taskcontext.DecodeStrict(raw, "cache-observation-v1.schema.json", &prior) != nil ||
			prior.Status == "started" || prior.FinishedAt == nil {
			return observation, errors.New("preceding attempt is incomplete")
		}
	}
	attempt := filepath.Join(report.Directory, "attempt-"+stepID)
	if err := os.Mkdir(attempt, 0700); err != nil {
		return observation, errors.New("step already reserved or attempt directory unavailable")
	}
	observation = CacheObservation{SchemaVersion: promptcache.ObservationVersion, StepID: stepID,
		Endpoint: endpoint, RequestDigest: report.Requests[stepID], StartedAt: time.Now().UTC(), Status: "started"}
	if err := writeTaskJSON(attempt, "attempt-start.json", observation); err != nil {
		return observation, err
	}
	defer func() {
		now := time.Now().UTC()
		observation.FinishedAt = &now
		if runErr != nil {
			message := runErr.Error()
			observation.Error = &message
			observation.Status = "failed"
		}
		if err := writeTaskJSON(attempt, "observation.json", observation); err != nil {
			runErr = err
		}
	}()
	requestBytes, err := readCacheFile(report.Directory, "request-"+stepID+".json")
	if err != nil {
		return observation, err
	}
	var request promptcache.Request
	if err := taskcontext.DecodeStrict(requestBytes, "cache-request-v1.schema.json", &request); err != nil {
		return observation, err
	}
	if err := writeTaskFile(attempt, "request.json", requestBytes); err != nil {
		return observation, err
	}
	if err := writeTaskFile(attempt, "submitted-request.json", []byte(request.WireBody)); err != nil {
		return observation, err
	}
	repo, err := repository.Open(ctx, report.Repository)
	if err != nil {
		return observation, err
	}
	capture, err := repo.CaptureEvidence(ctx)
	if err != nil {
		return observation, err
	}
	observation.Before = &capture.Evidence
	expected, err := cue.DecodeTask(fullCue)
	if err != nil {
		return observation, err
	}
	if capture.Evidence.Fingerprint != report.Basis.Fingerprint {
		return observation, errors.New("repository changed before request")
	}
	if err := validateCacheSources(ctx, repo, capture, expected); err != nil {
		return observation, err
	}
	key := os.Getenv("ANTHROPIC_API_KEY")
	result := claudeadapter.Send(ctx, endpoint, key, report.Spec.Provider.APIVersion, report.Spec.Limits, []byte(request.WireBody))
	observation.HTTPAttempted = result.Attempted
	observation.RequestWritten = result.RequestWritten
	if result.Attempted {
		observation.HTTPStartedAt = &result.StartedAt
		observation.HTTPFinishedAt = &result.FinishedAt
		observation.DurationMS = &result.DurationMS
	}
	observation.HTTPStatus = result.HTTPStatus
	observation.RequestID = result.RequestID
	observation.BodyComplete = result.BodyComplete
	if result.Body != nil {
		digest := taskDigest(result.Body)
		observation.ResponseDigest = &digest
		if err := writeTaskFile(attempt, "response.json", result.Body); err != nil {
			return observation, err
		}
	}
	after, afterErr := captureTaskBasis(repo)
	observation.After = after
	if result.Error != nil {
		return observation, result.Error
	}
	if afterErr != nil || after.Fingerprint != report.Basis.Fingerprint {
		return observation, errors.New("repository changed during request or post-request evidence unavailable")
	}
	ttl := report.Spec.Provider.TTL
	if report.Spec.Steps[index].Arm == "baseline" {
		ttl = ""
	}
	parsed, err := promptcache.DecodeResponse(result.Body, ttl)
	if err != nil {
		return observation, err
	}
	if parsed.Model != report.Spec.Provider.Model {
		return observation, errors.New("response model differs from frozen provider")
	}
	if parsed.Answer == nil {
		return observation, errors.New(parsed.AnswerError)
	}
	observation.Status = "completed"
	return observation, nil
}
