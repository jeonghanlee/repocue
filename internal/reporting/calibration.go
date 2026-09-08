package reporting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/docs/schema"
	"github.com/jeonghanlee/repocue/internal/benchmark"
	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/evaluation"
	"github.com/jeonghanlee/repocue/internal/evidence"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/storage"
)

type ImportConfig struct {
	Reports      string
	EvidenceFile string
	Assessments  string
}

func Import(ctx context.Context, repo *repository.Repository, store *storage.Store, config ImportConfig) (Calibration, storage.CalibrationReference, error) {
	state, err := store.Current(ctx, repo.ID)
	if err != nil {
		return Calibration{}, storage.CalibrationReference{}, err
	}
	before, err := repo.CaptureEvidence(ctx)
	if err != nil {
		return Calibration{}, storage.CalibrationReference{}, err
	}
	if before.Scan.RepositoryDigest != state.Snapshot.RepositoryDigest || before.Scan.Basis.StatusDigest != state.Snapshot.Basis.StatusDigest {
		return Calibration{}, storage.CalibrationReference{}, errors.New("refresh-needed: import requires a maintained current snapshot")
	}
	calibration, err := loadCalibration(config, repo, state, before)
	if err != nil {
		return Calibration{}, storage.CalibrationReference{}, err
	}
	after, err := repo.CaptureEvidence(ctx)
	if err != nil {
		return Calibration{}, storage.CalibrationReference{}, err
	}
	if before.Evidence.Fingerprint != after.Evidence.Fingerprint {
		return Calibration{}, storage.CalibrationReference{}, errors.New("repository changed during calibration import")
	}
	encoded, err := json.MarshalIndent(calibration, "", "  ")
	if err != nil {
		return Calibration{}, storage.CalibrationReference{}, err
	}
	encoded = append(encoded, '\n')
	if err := schema.Validate("calibration-v1.schema.json", encoded); err != nil {
		return Calibration{}, storage.CalibrationReference{}, err
	}
	ref := storage.CalibrationReference{ID: calibration.ID, RepositoryID: repo.ID, EpochID: state.Epoch.ID,
		SnapshotID: state.Snapshot.ID, ObservedAt: before.Evidence.ObservedAt, Fingerprint: before.Evidence.Fingerprint,
		Algorithm: before.Evidence.Algorithm, ContractDigest: calibration.ContractDigest,
		Model: calibration.Runner.Model, Benchmark: calibration.Runner.BenchmarkVersion}
	ref, err = store.PublishCalibration(ctx, state, ref, encoded)
	if err != nil {
		return Calibration{}, storage.CalibrationReference{}, err
	}
	// Idempotent imports return the original immutable observation, not new times.
	stored, err := evidence.ReadReference(filepath.Dir(store.Path()), evidence.FileReference{Path: ref.ArtifactPath, Digest: ref.Digest})
	if err != nil {
		return Calibration{}, storage.CalibrationReference{}, err
	}
	if err := decode(stored, &calibration); err != nil {
		return Calibration{}, storage.CalibrationReference{}, err
	}
	return calibration, ref, nil
}

func loadCalibration(config ImportConfig, repo *repository.Repository, state model.CurrentState, capture repository.EvidenceCapture) (Calibration, error) {
	serialized, err := os.ReadFile(config.EvidenceFile)
	if err != nil {
		return Calibration{}, err
	}
	if err := schema.Validate("calibration-evidence-v1.schema.json", serialized); err != nil {
		return Calibration{}, err
	}
	var proof evidence.Document
	if err := decode(serialized, &proof); err != nil {
		return Calibration{}, err
	}
	if proof.Capture.RepositoryID != repo.ID || !sameCapture(proof.Capture, capture.Evidence) ||
		!reflect.DeepEqual(proof.Profile, evidence.ProfileOf(capture.Scan)) {
		return Calibration{}, errors.New("source evidence does not match the target checkout and content profile")
	}
	calibration := Calibration{SchemaVersion: CalibrationSchema, EvidenceDigest: evidence.Digest(serialized),
		Binding:  Binding{RepositoryID: repo.ID, Epoch: state.Epoch.ID, Snapshot: state.Snapshot.ID, Evidence: capture.Evidence},
		Producer: proof.Producer, CueContract: proof.Cue, Profile: proof.Profile, SourceDigests: []string{}, Pairs: []Pair{},
		Comparisons: map[string]Comparison{}, SampleGrade: "exploratory", Limitations: calibrationLimitations()}
	records := map[string][]byte{}
	for _, ref := range proof.Records {
		if _, exists := records[ref.Digest]; exists {
			return Calibration{}, errors.New("duplicate external evidence record")
		}
		data, err := evidence.ReadReference(config.Reports, ref)
		if err != nil {
			return Calibration{}, err
		}
		records[ref.Digest] = data
	}
	byPair := map[string]*Pair{}
	byDigest := map[string]evaluation.M2ConditionReport{}
	seenSlots := map[string]bool{}
	var commonRepo, commonSchema string
	for _, source := range proof.Reports {
		if _, exists := byDigest[source.Digest]; exists {
			return Calibration{}, errors.New("duplicate selected report digest")
		}
		slot := source.PairID + "\x00" + source.Condition
		if seenSlots[slot] {
			return Calibration{}, errors.New("duplicate pair condition")
		}
		seenSlots[slot] = true
		if !sameCapture(source.Before, proof.Capture) || !sameCapture(source.After, proof.Capture) || source.After.ObservedAt.Before(source.Before.ObservedAt) {
			return Calibration{}, errors.New("selected report has incompatible pre/post content evidence")
		}
		data, err := evidence.ReadReference(config.Reports, source.FileReference)
		if err != nil {
			return Calibration{}, err
		}
		var report evaluation.M2ConditionReport
		if err := decode(data, &report); err != nil {
			return Calibration{}, err
		}
		schemaFile, err := evaluation.CalibrationReportSchema(report.SchemaVersion)
		if err != nil {
			return Calibration{}, err
		}
		if err := schema.Validate(schemaFile, data); err != nil {
			return Calibration{}, err
		}
		if commonSchema == "" {
			commonSchema = report.SchemaVersion
		} else if commonSchema != report.SchemaVersion {
			return Calibration{}, errors.New("mixed evaluation report schema versions")
		}
		if report.Condition != source.Condition || report.RunIndex != source.RunIndex || report.RepoCueSnapshot != source.Snapshot || report.Repository.StateFingerprint != source.StateFingerprint ||
			report.Repository.ID != repo.ID || report.Repository.ResolvedRoot != repo.Root || report.Repository.RepoCueStatusDigest != proof.Capture.StatusDigest || report.Repository.RepositoryDigest != proof.Capture.TrackedDigest ||
			report.Repository.TrackedFiles != proof.Profile.TrackedFiles || report.Repository.TrackedBytes != proof.Profile.TrackedBytes || report.BenchmarkVersion != benchmark.Version || report.OutputSchemaVersion != benchmark.AnswerSchemaVersion ||
			report.Cue.MaxTokens != proof.Cue.MaxTokens || (report.Condition == evaluation.ConditionBasic && report.Cue.SchemaVersion != proof.Cue.SchemaVersion) {
			return Calibration{}, errors.New("selected report does not match calibration contracts")
		}
		if err := validateOperationCosts(report); err != nil {
			return Calibration{}, err
		}
		basis := model.Basis{Head: report.Repository.Head}
		var scoreEvidence *evidence.Scoring
		for index := range proof.Scoring {
			item := &proof.Scoring[index]
			if item.ReportDigest == source.Digest {
				if scoreEvidence != nil {
					return Calibration{}, errors.New("duplicate scoring evidence")
				}
				scoreEvidence = item
				basis = item.Basis
			}
		}
		if err := evaluation.ValidateCalibrationReport(report, basis); err != nil {
			return Calibration{}, err
		}
		identity := report.Runner.Metadata
		identity.RepoCueSnapshot = ""
		if len(byDigest) == 0 {
			calibration.Runner = identity
		} else if identity != calibration.Runner {
			return Calibration{}, errors.New("mixed runner/model/reasoning/benchmark contract")
		}
		repoBasis := report.Repository
		repoBasis.SuppliedPath = ""
		repoBasis.ResolvedRoot = ""
		repoIdentity, err := evidence.Identity(repoBasis)
		if err != nil {
			return Calibration{}, err
		}
		if commonRepo == "" {
			commonRepo = repoIdentity
		} else if commonRepo != repoIdentity {
			return Calibration{}, errors.New("mixed repository state")
		}
		sample, err := sampleOf(report, source)
		if err != nil {
			return Calibration{}, err
		}
		if scoreEvidence != nil {
			left, right := scoreEvidence.Basis, capture.Scan.Basis
			left.ObservedAt = time.Time{}
			right.ObservedAt = time.Time{}
			if !reflect.DeepEqual(left, right) || scoreEvidence.ScorerRevision != proof.Producer.Revision || scoreEvidence.ScoredAt.Before(source.Before.ObservedAt) {
				return Calibration{}, errors.New("scoring basis/provenance contradicts selected repository evidence")
			}
			_, score, err := benchmark.ParseAndScore(report.Runner.FinalResponse, scoreEvidence.Basis)
			if err != nil {
				return Calibration{}, err
			}
			if (scoreEvidence.Passed != nil && *scoreEvidence.Passed != score.Passed) || (scoreEvidence.Total != nil && *scoreEvidence.Total != score.Total) {
				return Calibration{}, errors.New("supplied correctness totals contradict the shipped scorer")
			}
			sample.Correctness = Correctness{Status: "derived", Passed: &score.Passed, Total: &score.Total, Ratio: &score.Ratio,
				Facts: map[string]bool{}, ScorerRevision: scoreEvidence.ScorerRevision, ScoredAt: &scoreEvidence.ScoredAt}
			for _, fact := range score.Facts {
				sample.Correctness.Facts[fact.Fact] = fact.Passed
			}
		}
		pair := byPair[source.PairID]
		if pair == nil {
			pair = &Pair{ID: source.PairID, MaintenanceID: report.MaintenanceID, Baseline: experimentCost(report.Baseline), Update: experimentCost(report.Update), Cue: map[string]Measurement{}}
			byPair[source.PairID] = pair
		} else if pair.MaintenanceID != report.MaintenanceID || !reflect.DeepEqual(pair.Baseline, experimentCost(report.Baseline)) || !reflect.DeepEqual(pair.Update, experimentCost(report.Update)) {
			return Calibration{}, errors.New("pair maintenance evidence differs")
		}
		if source.Condition == evaluation.ConditionDirect {
			pair.Direct = sample
		} else {
			pair.Basic = sample
			pair.Cue = map[string]Measurement{
				"execution_duration_ms": missing("ms", "full shipped cue CLI", "full-path observation unavailable"),
				"git_invocations":       missing("Git processes", "full shipped cue CLI", "process observation unavailable"),
				"builder_duration_ms":   measured("observed", report.Cue.WallDurationMS, "ms", "in-process builder only; not added to full CLI cost"),
				"output_bytes":          measured("observed", float64(report.Cue.Bytes), "bytes", "selected evaluation cue"),
				"estimated_tokens":      measured("estimated", float64(report.Cue.EstimatedTokens), "estimated tokens", "diagnostic only; already included in agent input"),
			}
		}
		byDigest[source.Digest] = report
		calibration.SourceDigests = append(calibration.SourceDigests, source.Digest)
	}
	for _, item := range proof.Scoring {
		if _, found := byDigest[item.ReportDigest]; !found {
			return Calibration{}, errors.New("scoring reference has no selected report")
		}
	}
	seenCLI := map[string]bool{}
	for _, cli := range proof.CueCLI {
		pair := byPair[cli.PairID]
		if pair == nil || seenCLI[cli.PairID] {
			return Calibration{}, errors.New("unknown or duplicate cue CLI pair")
		}
		seenCLI[cli.PairID] = true
		report := byDigest[pair.Basic.ReportDigest]
		if cli.ExecutableDigest != proof.Producer.ExecutableDigest || cli.Snapshot != pair.Basic.SourceSnapshot || cli.CompletedAt.Before(cli.StartedAt) || cli.OutputBytes != report.Cue.Bytes || cli.EstimatedTokens != report.Cue.EstimatedTokens {
			return Calibration{}, errors.New("cue CLI evidence does not match assisted report")
		}
		if cli.CompletedAt.After(pair.Basic.ObservedAt) {
			return Calibration{}, errors.New("cue CLI observation must finish before the assisted pre-run capture")
		}
		content, exists := records[cli.OutputDigest]
		if !exists || len(content) != cli.OutputBytes {
			return Calibration{}, errors.New("measured cue stdout evidence is missing")
		}
		if err := schema.Validate("cue-v1.schema.json", content); err != nil {
			return Calibration{}, err
		}
		var envelope cue.Envelope
		if err := json.Unmarshal(content, &envelope); err != nil {
			return Calibration{}, err
		}
		if envelope.Kind != proof.Cue.View || envelope.Repository.ID != repo.ID || envelope.Snapshot != cli.Snapshot ||
			envelope.Basis.StatusDigest != proof.Capture.StatusDigest || envelope.Basis.RepositoryDigest != proof.Capture.TrackedDigest ||
			envelope.Budget.MaxTokens != proof.Cue.MaxTokens || envelope.Budget.EstimatedTokens != cli.EstimatedTokens ||
			envelope.Budget.EstimatedTokens > proof.Cue.MaxTokens || envelope.Freshness != cli.Freshness {
			return Calibration{}, errors.New("captured cue envelope contradicts source basis or budget")
		}
		if len(cli.Arguments) != 9 || cli.Arguments[0] != "cue" || cli.Arguments[1] != "--repository" || cli.Arguments[2] != repo.Root ||
			cli.Arguments[3] != "--cache-dir" || !filepath.IsAbs(cli.Arguments[4]) || cli.Arguments[5] != "--view" || cli.Arguments[6] != proof.Cue.View ||
			cli.Arguments[7] != "--max-tokens" || cli.Arguments[8] != strconv.Itoa(proof.Cue.MaxTokens) ||
			math.Abs(cli.DurationMS-float64(cli.CompletedAt.Sub(cli.StartedAt))/float64(time.Millisecond)) > 0.001 {
			return Calibration{}, errors.New("cue CLI arguments or duration contradict retained observation")
		}
		pair.Cue["execution_duration_ms"] = measured("observed", cli.DurationMS, "ms", "full shipped cue CLI, launch through captured stdout and exit")
		pair.Cue["git_invocations"] = missing("Git processes", "full shipped cue CLI", "Git process observation unavailable")
		if cli.GitStatus == "observed" {
			if cli.GitInvocations == nil {
				return Calibration{}, errors.New("observed Git CLI count is missing")
			}
			count, err := evaluation.CountGitInvocations(records[cli.GitTraceDigest])
			if err != nil || count != *cli.GitInvocations {
				return Calibration{}, errors.New("Git process count lacks complete retained events")
			}
			pair.Cue["git_invocations"] = measured("observed", float64(*cli.GitInvocations), "Git processes", "full cue CLI Git Trace2 process sessions")
		} else if cli.GitInvocations != nil {
			return Calibration{}, errors.New("unobserved Git CLI count has a value")
		}
		pair.Cue["output_bytes"] = measured("observed", float64(cli.OutputBytes), "bytes", "captured full cue stdout")
		pair.Cue["estimated_tokens"] = measured("estimated", float64(cli.EstimatedTokens), "estimated tokens", "runtime cue estimate, not additional agent input")
		pair.CueObservedAt = &cli.StartedAt
	}
	if err := joinAssessments(config, proof, byDigest, byPair); err != nil {
		return Calibration{}, err
	}
	ids := make([]string, 0, len(byPair))
	for id := range byPair {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	logicalPairs := map[string]bool{}
	for _, id := range ids {
		pair := byPair[id]
		if pair.Direct.ReportDigest == "" || pair.Basic.ReportDigest == "" || pair.Direct.RunIndex != pair.Basic.RunIndex {
			return Calibration{}, errors.New("incomplete or incorrectly paired direct/basic set")
		}
		identity := pair.MaintenanceID + "/" + strconv.Itoa(pair.Direct.RunIndex)
		if logicalPairs[identity] {
			return Calibration{}, errors.New("duplicate logical pair")
		}
		logicalPairs[identity] = true
		calibration.Pairs = append(calibration.Pairs, *pair)
	}
	if len(calibration.Pairs) >= 3 {
		calibration.SampleGrade = "descriptive"
	}
	sort.Strings(calibration.SourceDigests)
	contract, err := evidence.Identity(struct {
		Producer evidence.Producer
		Cue      evidence.CueContract
		Runner   evaluation.RunnerMetadata
	}{proof.Producer, proof.Cue, calibration.Runner})
	if err != nil {
		return Calibration{}, err
	}
	calibration.ContractDigest = contract
	identity, err := evidence.Identity(struct {
		Evidence                                 string
		Reports                                  []string
		Repository, Epoch, Snapshot, Fingerprint string
		Assessments                              bool
	}{calibration.EvidenceDigest, calibration.SourceDigests, repo.ID, state.Epoch.ID, state.Snapshot.ID, capture.Evidence.Fingerprint, config.Assessments != ""})
	if err != nil {
		return Calibration{}, err
	}
	calibration.ID = "calibration-" + strings.TrimPrefix(identity, "sha256:")
	for _, metric := range agentMetricNames() {
		calibration.Comparisons[metric] = comparePairs(calibration.Pairs, metric)
	}
	return calibration, nil
}

func sameCapture(left, right repository.ContentEvidence) bool {
	return left.Complete && right.Complete && left.Algorithm == right.Algorithm && left.RepositoryID == right.RepositoryID && left.Fingerprint == right.Fingerprint &&
		left.StatusDigest == right.StatusDigest && left.TrackedDigest == right.TrackedDigest && left.UntrackedDigest == right.UntrackedDigest
}

func decode(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("expected one complete JSON document")
	}
	return nil
}

func validateOperationCosts(report evaluation.M2ConditionReport) error {
	for _, cost := range []evaluation.OperationCost{report.Baseline, report.Update} {
		if cost.WallDurationMS < 0 || cost.ScanDurationMS < 0 || cost.FilesScanned < 0 || cost.BytesScanned < 0 || cost.GitCommands < 0 || cost.DatabaseSizeBytes < 0 || cost.MaterializedRows < 0 {
			return errors.New("negative calibration maintenance cost")
		}
	}
	if report.Cue.Bytes < 0 || report.Cue.EstimatedTokens < 0 || report.Cue.WallDurationMS < 0 {
		return errors.New("negative cue cost")
	}
	return nil
}

func sampleOf(report evaluation.M2ConditionReport, source evidence.SourceReport) (Sample, error) {
	metrics := map[string]Measurement{}
	encoded, err := json.Marshal(report.Runner.Metrics)
	if err != nil {
		return Sample{}, err
	}
	values := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &values); err != nil {
		return Sample{}, err
	}
	for _, name := range agentMetricNames() {
		unit, basis := metricBoundary(name)
		value := missing(unit, basis, "agent-measurement-unavailable")
		if raw, found := values[name]; found {
			var number float64
			if json.Unmarshal(raw, &number) == nil && !bytes.Equal(raw, []byte("null")) {
				status := report.Runner.Metrics.Statuses[name]
				if status == "observed" || status == "derived" || status == "estimated" {
					value = measured(status, number, unit, basis)
				}
			}
		}
		metrics[name] = value
	}
	return Sample{ReportDigest: source.Digest, Condition: source.Condition, RunIndex: source.RunIndex, SourceSnapshot: source.Snapshot,
		Head: report.Repository.Head, StateFingerprint: source.StateFingerprint, ObservedAt: source.Before.ObservedAt, Metrics: metrics,
		Correctness: Correctness{Status: "not_observed", Facts: map[string]bool{}},
		Assessment:  Assessment{Status: "not_observed", TimeStatus: "not_observed", Scores: map[string]int{}}}, nil
}

func experimentCost(cost evaluation.OperationCost) map[string]Measurement {
	return map[string]Measurement{
		"execution_duration_ms":  measured("observed", cost.WallDurationMS, "ms", "calibration experiment operation, not local H"),
		"scan_duration_ms":       measured("observed", cost.ScanDurationMS, "ms", "scan portion included in experiment duration"),
		"files_scanned":          measured("observed", float64(cost.FilesScanned), "files", "experiment tracked-file scan"),
		"bytes_scanned":          measured("observed", float64(cost.BytesScanned), "scan bytes", "experiment content bytes, not filesystem I/O"),
		"git_invocations":        measured("observed", float64(cost.GitCommands), "Git processes", "experiment core scan invocations"),
		"database_size_bytes":    measured("observed", float64(cost.DatabaseSizeBytes), "bytes", "experiment database point measurement"),
		"materialized_file_rows": measured("derived", float64(cost.MaterializedRows), "rows", "experiment snapshot file rows"),
	}
}

func agentMetricNames() []string {
	return []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens", "execution_duration_ms",
		"command_executions", "repository_files_named", "named_file_size_proxy_bytes", "git_calls", "filesystem_search_calls", "tool_calls",
		"fallback_repository_commands", "fallback_repository_files_named", "fallback_named_file_size_proxy_bytes", "git_invocations"}
}

func calibrationLimitations() []string {
	return []string{"calibration is a dated reference, not observed current consumer savings", "three pairs describe ranges, not statistical significance",
		"named file sizes are proxies, not filesystem I/O", "Git command strings are not Git process counts", "quality is not renewed for later snapshots",
		"ignored files and intermediate writes are outside content evidence; gitlinks retain non-recursive core coverage"}
}

func joinAssessments(config ImportConfig, proof evidence.Document, reports map[string]evaluation.M2ConditionReport, pairs map[string]*Pair) error {
	if config.Assessments == "" {
		return nil
	}
	seenReports, seenResponses := map[string]bool{}, map[string]bool{}
	assessor := ""
	for _, ref := range proof.Assessments {
		report, exists := reports[ref.ReportDigest]
		if !exists || seenReports[ref.ReportDigest] || seenResponses[ref.ResponseID] {
			return errors.New("ambiguous qualitative assessment join")
		}
		seenReports[ref.ReportDigest] = true
		seenResponses[ref.ResponseID] = true
		data, err := evidence.ReadReference(config.Assessments, ref.FileReference)
		if err != nil {
			return err
		}
		if err := schema.Validate("qualitative-assessment-v2.schema.json", data); err != nil {
			return err
		}
		var parsed struct {
			SchemaVersion string         `json:"schema_version"`
			Assessor      string         `json:"assessor"`
			ResponseID    string         `json:"response_id"`
			Condition     string         `json:"condition"`
			Head          *string        `json:"repository_head"`
			Fingerprint   string         `json:"state_fingerprint"`
			RunIndex      int            `json:"run_index"`
			Scores        map[string]int `json:"scores"`
			Evidence      []string       `json:"evidence"`
		}
		if err := decode(data, &parsed); err != nil {
			return err
		}
		if parsed.ResponseID != ref.ResponseID || parsed.Condition != report.Condition || parsed.RunIndex != report.RunIndex || !reflect.DeepEqual(parsed.Head, report.Repository.Head) || parsed.Fingerprint != report.Repository.StateFingerprint {
			return errors.New("qualitative assessment basis mismatch")
		}
		if assessor != "" && assessor != parsed.Assessor {
			return errors.New("mixed qualitative assessors")
		}
		assessor = parsed.Assessor
		assessment := Assessment{Status: "observed", ResponseID: parsed.ResponseID, Assessor: parsed.Assessor, Rubric: parsed.SchemaVersion, Digest: ref.Digest, Scores: parsed.Scores, AssessedAt: ref.AssessedAt, TimeStatus: "not_observed"}
		if ref.AssessedAt != nil {
			assessment.TimeStatus = "observed"
		}
		for _, pair := range pairs {
			if pair.Direct.ReportDigest == ref.ReportDigest {
				pair.Direct.Assessment = assessment
			}
			if pair.Basic.ReportDigest == ref.ReportDigest {
				pair.Basic.Assessment = assessment
			}
		}
	}
	return nil
}
