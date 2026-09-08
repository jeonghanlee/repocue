package evaluation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/docs/schema"
	"github.com/jeonghanlee/repocue/internal/benchmark"
	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/evidence"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
)

func prepareCalibrationContext(ctx context.Context, config M2Config, repo *repository.Repository, state model.CurrentState, workspace, freshness string) (map[string][]byte, map[string]M2CueCost, *evidence.Document, error) {
	if config.CueExecutable == "" {
		path, err := os.Executable()
		if err != nil {
			return nil, nil, nil, err
		}
		config.CueExecutable = path
	}
	producer, err := evidence.ExecutableProducer(config.CueExecutable)
	if err != nil {
		return nil, nil, nil, err
	}
	capture, err := repo.CaptureEvidence(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	if capture.Scan.RepositoryDigest != state.Snapshot.RepositoryDigest || capture.Scan.Basis.StatusDigest != state.Snapshot.Basis.StatusDigest {
		return nil, nil, nil, errors.New("calibration capture does not match maintained snapshot")
	}
	started := time.Now()
	_, _, err = cue.Overview(state, freshness, config.MaxTokens)
	if err != nil {
		return nil, nil, nil, err
	}
	builderMS := durationMS(time.Since(started))
	content, observation, trace, err := observeCueCLI(ctx, config.CueExecutable, producer, repo, state, filepath.Join(workspace, "cache"), config.MaxTokens)
	if err != nil {
		return nil, nil, nil, err
	}
	after, err := repo.CaptureEvidence(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	if after.Evidence.Fingerprint != capture.Evidence.Fingerprint {
		return nil, nil, nil, errors.New("repository changed during full cue CLI measurement")
	}
	proof := &evidence.Document{
		SchemaVersion: evidence.SchemaVersion, Producer: producer,
		Cue:     evidence.CueContract{View: "overview", SchemaVersion: model.SchemaVersion, MaxTokens: config.MaxTokens, SelectionPolicy: evidence.SelectionPolicy},
		Profile: evidence.ProfileOf(capture.Scan), Capture: capture.Evidence,
		Reports: []evidence.SourceReport{}, Scoring: []evidence.Scoring{},
		CueCLI: []evidence.CueCLI{observation}, Assessments: []evidence.AssessmentReference{},
		Records:   []evidence.FileReference{{Path: "basic-cue.json", Digest: evidence.Digest(content)}},
		Artifacts: map[string][]byte{"basic-cue.json": content},
	}
	if len(trace) > 0 {
		proof.Records = append(proof.Records, evidence.FileReference{Path: "cue-git-events.jsonl", Digest: evidence.Digest(trace)})
		proof.Artifacts["cue-git-events.jsonl"] = trace
	}
	costs := map[string]M2CueCost{
		ConditionDirect: {MaxTokens: config.MaxTokens},
		ConditionBasic:  {SchemaVersion: model.SchemaVersion, Bytes: len(content), EstimatedTokens: observation.EstimatedTokens, MaxTokens: config.MaxTokens, WallDurationMS: builderMS},
	}
	return map[string][]byte{ConditionDirect: nil, ConditionBasic: content}, costs, proof, nil
}

func observeCueCLI(ctx context.Context, executable string, producer evidence.Producer, repo *repository.Repository, state model.CurrentState, cache string, maxTokens int) ([]byte, evidence.CueCLI, []byte, error) {
	args := []string{"cue", "--repository", repo.Root, "--cache-dir", cache, "--view", "overview", "--max-tokens", strconv.Itoa(maxTokens)}
	observation := evidence.CueCLI{ExecutableDigest: producer.ExecutableDigest, Arguments: args,
		CacheState: "initialized", LocationHintState: "absent", GitStatus: "not_observed"}
	if _, err := os.Stat(filepath.Join(cache, repo.ID, "location.json")); err == nil {
		observation.LocationHintState = "present"
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, observation, nil, err
	}
	traceFile, err := os.CreateTemp(filepath.Dir(cache), "cue-git-*.jsonl")
	if err != nil {
		return nil, observation, nil, err
	}
	tracePath := traceFile.Name()
	if err := traceFile.Close(); err != nil {
		return nil, observation, nil, err
	}
	defer os.Remove(tracePath)
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = repo.Root
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GIT_TRACE2_") && !strings.HasPrefix(value, "GIT_OPTIONAL_LOCKS=") {
			command.Env = append(command.Env, value)
		}
	}
	// Core status already disables optional locks. An extra Git environment
	// override would disable the location hint and change the measured path.
	command.Env = append(command.Env, "GIT_TRACE2_EVENT="+tracePath)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	observation.StartedAt = time.Now().UTC()
	err = command.Run()
	observation.CompletedAt = time.Now().UTC()
	observation.DurationMS = durationMS(observation.CompletedAt.Sub(observation.StartedAt))
	if err != nil {
		return nil, observation, nil, fmt.Errorf("measure shipped cue CLI: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	content := stdout.Bytes()
	if err := schema.Validate("cue-v1.schema.json", content); err != nil {
		return nil, observation, nil, err
	}
	var envelope cue.Envelope
	if err := json.Unmarshal(content, &envelope); err != nil {
		return nil, observation, nil, err
	}
	if envelope.Kind != "overview" || envelope.Repository.ID != repo.ID || envelope.Snapshot != state.Snapshot.ID ||
		envelope.Epoch != state.Epoch.ID || envelope.Basis.StatusDigest != state.Snapshot.Basis.StatusDigest ||
		envelope.Basis.RepositoryDigest != state.Snapshot.RepositoryDigest || envelope.Budget.MaxTokens != maxTokens ||
		envelope.Budget.EstimatedTokens > maxTokens || (envelope.Freshness != "current" && envelope.Freshness != "dirty-but-indexed") {
		return nil, observation, nil, errors.New("full cue CLI output does not match evaluation basis")
	}
	observation.OutputDigest, observation.OutputBytes = evidence.Digest(content), len(content)
	observation.EstimatedTokens, observation.Snapshot, observation.Freshness = envelope.Budget.EstimatedTokens, envelope.Snapshot, envelope.Freshness
	trace, err := os.ReadFile(tracePath)
	if err != nil {
		return nil, observation, nil, err
	}
	if count, err := CountGitInvocations(trace); err == nil {
		observation.GitInvocations, observation.GitStatus = &count, "observed"
		observation.GitTraceDigest = evidence.Digest(trace)
	}
	return content, observation, trace, nil
}

// CountGitInvocations counts complete Git Trace2 process sessions, not shell
// command strings or RepoCue's builder-only diagnostic count.
func CountGitInvocations(trace []byte) (int, error) {
	starts, versions, exits := map[string]bool{}, map[string]bool{}, map[string]bool{}
	scanner := bufio.NewScanner(bytes.NewReader(trace))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for scanner.Scan() {
		var event struct {
			Event string `json:"event"`
			SID   string `json:"sid"`
			Exe   string `json:"exe"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil || event.SID == "" {
			return 0, errors.New("incomplete Git process trace")
		}
		switch event.Event {
		case "version":
			versions[event.SID] = event.Exe != ""
		case "start":
			if starts[event.SID] {
				return 0, errors.New("duplicate Git process trace")
			}
			starts[event.SID] = true
		case "exit":
			exits[event.SID] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if len(starts) == 0 || len(starts) != len(versions) || len(starts) != len(exits) {
		return 0, errors.New("Git process coverage is not observed")
	}
	for sid := range starts {
		if !versions[sid] || !exits[sid] {
			return 0, errors.New("Git process coverage is incomplete")
		}
	}
	return len(starts), nil
}

func appendCalibrationReport(proof *evidence.Document, report M2ConditionReport, pairID string, before, after repository.EvidenceCapture) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	modelName := "no-runner"
	if report.Runner != nil {
		modelName = report.Runner.Metadata.Model
	}
	digest := evidence.Digest(encoded)
	proof.Reports = append(proof.Reports, evidence.SourceReport{
		FileReference: evidence.FileReference{Path: m2ReportName(report, modelName), Digest: digest},
		PairID:        pairID, Condition: report.Condition, RunIndex: report.RunIndex, Snapshot: report.RepoCueSnapshot,
		StateFingerprint: report.Repository.StateFingerprint, Before: before.Evidence, After: after.Evidence,
	})
	if report.Runner != nil {
		_, score, err := benchmark.ParseAndScore(report.Runner.FinalResponse, before.Scan.Basis)
		if err != nil {
			return err
		}
		proof.Scoring = append(proof.Scoring, evidence.Scoring{ReportDigest: digest, Basis: before.Scan.Basis,
			ScorerRevision: proof.Producer.Revision, ScoredAt: time.Now().UTC(), Passed: &score.Passed, Total: &score.Total})
	}
	return nil
}

func writeCalibrationEvidence(directory string, proof *evidence.Document, cueBytes []byte) error {
	if len(proof.CueCLI) != 1 || evidence.Digest(cueBytes) != proof.CueCLI[0].OutputDigest {
		return errors.New("captured cue output does not match delivered bytes")
	}
	encoded, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := schema.Validate("calibration-evidence-v1.schema.json", encoded); err != nil {
		return err
	}
	for _, ref := range proof.Records {
		content, found := proof.Artifacts[ref.Path]
		if !found || evidence.Digest(content) != ref.Digest || !filepath.IsLocal(ref.Path) {
			return errors.New("missing captured calibration artifact")
		}
		if err := os.WriteFile(filepath.Join(directory, ref.Path), content, 0o600); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(directory, "calibration-evidence.json"), encoded, 0o600)
}

// ValidateCalibrationReport reuses the shipped runner's full runtime validation
// for imported reports, with a contemporaneous scoring basis when available.
func ValidateCalibrationReport(report M2ConditionReport, basis model.Basis) error {
	if err := validateM2Report(report); err != nil {
		return err
	}
	if report.Runner == nil {
		return errors.New("runnerless reports cannot calibrate agent savings")
	}
	version, err := runnerSchemaVersion(report.SchemaVersion)
	if err != nil {
		return err
	}
	return validateM2ObservationVersion(*report.Runner, report.Condition, report.RunIndex, model.Snapshot{ID: report.RepoCueSnapshot, Basis: basis}, version)
}

// CalibrationReportSchema selects the immutable contract declared by a report.
func CalibrationReportSchema(version string) (string, error) {
	switch version {
	case M2SchemaVersion:
		return "evaluation-v4.schema.json", nil
	case LegacyM2SchemaVersion:
		return "evaluation-v3.schema.json", nil
	default:
		return "", errors.New("unsupported evaluation report schema version")
	}
}

func runnerSchemaVersion(version string) (string, error) {
	switch version {
	case M2SchemaVersion:
		return M2RunnerSchemaVersion, nil
	case LegacyM2SchemaVersion:
		return legacyRunnerSchemaVersion, nil
	default:
		return "", errors.New("unsupported evaluation report schema version")
	}
}
