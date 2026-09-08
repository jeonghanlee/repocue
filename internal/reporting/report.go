package reporting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/docs/schema"
	"github.com/jeonghanlee/repocue/internal/benchmark"
	"github.com/jeonghanlee/repocue/internal/evidence"
	"github.com/jeonghanlee/repocue/internal/metrics"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/storage"
)

type ReportConfig struct {
	GeneratedAt time.Time
	Cutoff      time.Time
	WindowDays  int64
	Historical  bool
	Profile     string
	Consumers   int64
	Producer    *evidence.Producer
}

type SelectedState struct {
	RepositoryID string    `json:"repository_id"`
	Epoch        string    `json:"epoch"`
	Snapshot     string    `json:"snapshot"`
	Branch       *string   `json:"branch"`
	Head         *string   `json:"head"`
	Dirty        bool      `json:"dirty"`
	ObservedAt   time.Time `json:"observed_at"`
}

type ProfileCandidate struct {
	ID         string    `json:"id"`
	Model      string    `json:"model"`
	Benchmark  string    `json:"benchmark"`
	ObservedAt time.Time `json:"observed_at"`
}

type Applicability struct {
	Status           string         `json:"status"`
	Reasons          []string       `json:"reasons"`
	TrackedFileDrift int            `json:"tracked_file_drift"`
	TrackedByteDrift int64          `json:"tracked_byte_drift"`
	LanguageDrift    map[string]int `json:"language_drift"`
}

type Quality struct {
	Status                string               `json:"status"`
	DirectCorrectness     Statistic            `json:"direct_correctness"`
	BasicCorrectness      Statistic            `json:"basic_correctness"`
	CorrectnessDifference Statistic            `json:"correctness_difference"`
	DirectFactPassCounts  map[string]int       `json:"direct_fact_pass_counts"`
	BasicFactPassCounts   map[string]int       `json:"basic_fact_pass_counts"`
	Qualitative           map[string]Statistic `json:"qualitative_basic_minus_direct"`
	Limitations           []string             `json:"limitations"`
}

type Report struct {
	SchemaVersion          string                     `json:"schema_version"`
	GeneratedAt            time.Time                  `json:"generated_at"`
	Window                 Window                     `json:"window"`
	Historical             bool                       `json:"historical"`
	DatabaseSchema         int                        `json:"database_schema"`
	State                  *SelectedState             `json:"state"`
	Freshness              string                     `json:"freshness"`
	LiveEvidence           repository.ContentEvidence `json:"live_evidence"`
	DatabaseSize           Measurement                `json:"database_size"`
	DatabaseSizeObservedAt time.Time                  `json:"database_size_observed_at"`
	Profiles               []ProfileCandidate         `json:"profiles"`
	Calibration            *Calibration               `json:"calibration"`
	CalibrationDigest      string                     `json:"calibration_digest"`
	Applicability          Applicability              `json:"applicability"`
	Activity               []EpochActivity            `json:"activity"`
	Maintenance            Maintenance                `json:"maintenance"`
	Projections            []Projection               `json:"projections"`
	Quality                Quality                    `json:"quality"`
	Unavailable            map[string]Measurement     `json:"unavailable"`
	TimeMeanings           map[string]string          `json:"time_meanings"`
	Warnings               []string                   `json:"warnings"`
	Limitations            []string                   `json:"limitations"`
}

func Generate(ctx context.Context, repo *repository.Repository, store *storage.Store, config ReportConfig) (Report, error) {
	if config.WindowDays < 1 || config.WindowDays > int64((1<<63-1)/int64(48*time.Hour)) {
		return Report{}, errors.New("report window is outside the supported positive day range")
	}
	counts, err := consumerCounts(config.Consumers)
	if err != nil {
		return Report{}, err
	}
	if config.GeneratedAt.IsZero() {
		config.GeneratedAt = time.Now().UTC()
	}
	if config.Cutoff.IsZero() {
		config.Cutoff = config.GeneratedAt
	}
	_, offset := config.Cutoff.Zone()
	if offset != 0 || config.Cutoff.After(config.GeneratedAt) {
		return Report{}, errors.New("report cutoff must be a non-future UTC instant")
	}
	duration := time.Duration(config.WindowDays) * 24 * time.Hour
	window := Window{Start: config.Cutoff.Add(-duration), Cutoff: config.Cutoff, PreviousStart: config.Cutoff.Add(-2 * duration), Days: config.WindowDays}
	if window.PreviousStart.Year() < 1 {
		return Report{}, errors.New("report window precedes supported calendar range")
	}
	history, err := store.ReadHistory(ctx, repo.ID, config.Cutoff)
	if err != nil {
		return Report{}, err
	}
	live, err := repo.CaptureEvidence(ctx)
	if err != nil {
		return Report{}, err
	}
	size, err := metrics.DatabaseSize(store.Path())
	if err != nil {
		return Report{}, err
	}
	result := Report{SchemaVersion: ReportSchema, GeneratedAt: config.GeneratedAt, Window: window, Historical: config.Historical, DatabaseSchema: history.SchemaVersion,
		Freshness: "refresh-needed", LiveEvidence: live.Evidence, DatabaseSize: measured("observed", float64(size), "bytes", "current DB files, not historical storage size"), DatabaseSizeObservedAt: time.Now().UTC(),
		Profiles: []ProfileCandidate{}, Applicability: Applicability{Status: "not_applicable", Reasons: []string{"no-calibration"}, LanguageDrift: map[string]int{}},
		Activity: activity(history, window), Maintenance: maintenance(history, history.Selected, config.Cutoff), Projections: []Projection{},
		Quality: quality(nil), Unavailable: unavailableMeasurements(), TimeMeanings: timeMeanings(), Warnings: []string{}, Limitations: calibrationLimitations()}
	if live.Scan.RepositoryDigest == history.Current.Snapshot.RepositoryDigest && live.Scan.Basis.StatusDigest == history.Current.Snapshot.Basis.StatusDigest {
		result.Freshness = "current"
		if live.Scan.Basis.Dirty {
			result.Freshness = "dirty-but-indexed"
		}
	}
	if history.Selected != nil {
		selected := history.Selected
		result.State = &SelectedState{RepositoryID: repo.ID, Epoch: selected.Epoch.ID, Snapshot: selected.Snapshot.ID, Branch: selected.Snapshot.Basis.Branch, Head: selected.Snapshot.Basis.Head, Dirty: selected.Snapshot.Basis.Dirty, ObservedAt: selected.Snapshot.Basis.ObservedAt}
	} else {
		result.Warnings = append(result.Warnings, "historical-state-unavailable")
	}
	eligible := []storage.CalibrationReference{}
	var chosen *storage.CalibrationReference
	for _, ref := range history.Calibrations {
		if ref.ObservedAt.Before(config.Cutoff) {
			eligible = append(eligible, ref)
			result.Profiles = append(result.Profiles, ProfileCandidate{ID: ref.ID, Model: ref.Model, Benchmark: ref.Benchmark, ObservedAt: ref.ObservedAt})
		}
		if config.Profile == ref.ID {
			value := ref
			chosen = &value
		}
	}
	if config.Profile != "" && chosen == nil {
		return Report{}, errors.New("unknown calibration profile")
	}
	if config.Profile == "" {
		if len(eligible) == 1 {
			chosen = &eligible[0]
		}
		if len(eligible) > 1 {
			result.Warnings = append(result.Warnings, "profile-selection-required")
		}
	}
	if chosen == nil {
		return result, nil
	}
	calibration, err := readCalibration(store, *chosen)
	if err != nil {
		return Report{}, err
	}
	result.Calibration = &calibration
	result.CalibrationDigest = chosen.Digest
	result.Quality = quality(calibration.Pairs)
	result.Applicability = applicability(calibration, *chosen, history, live, config, result.Freshness)
	if result.Quality.DirectCorrectness.Count < len(calibration.Pairs) || result.Quality.BasicCorrectness.Count < len(calibration.Pairs) {
		result.Warnings = append(result.Warnings, "deterministic-correctness-incomplete")
	}
	if result.Quality.Status == "not_observed" {
		result.Warnings = append(result.Warnings, "quality-not-assessed")
	}
	for _, pair := range calibration.Pairs {
		for _, sample := range []Sample{pair.Direct, pair.Basic} {
			if sample.Correctness.Passed != nil && sample.Correctness.Total != nil && *sample.Correctness.Passed < *sample.Correctness.Total {
				result.Warnings = append(result.Warnings, "deterministic-fact-failures")
				break
			}
		}
	}
	if result.Applicability.Status == "not_applicable" {
		result.Warnings = append(result.Warnings, "calibration-not-applicable")
		return result, nil
	}
	for _, metric := range []string{"input_tokens", "cached_input_tokens", "output_tokens", "reasoning_output_tokens", "total_tokens", "execution_duration_ms", "command_executions", "git_calls", "filesystem_search_calls", "tool_calls", "repository_files_named", "git_invocations"} {
		unit, basis := metricBoundary(metric)
		h := measured("derived", 0, unit, "agent-only comparison; RepoCue internals excluded")
		if value, found := result.Maintenance.Total[metric]; found {
			h = value
		}
		if metric == "git_invocations" {
			h = result.Maintenance.Total[metric]
			h.Basis = basis + "; " + h.Basis
		}
		for _, count := range counts {
			result.Projections = append(result.Projections, project(calibration.Pairs, metric, count, config.Consumers == count, h))
		}
	}
	sort.Strings(result.Warnings)
	return result, nil
}

func readCalibration(store *storage.Store, ref storage.CalibrationReference) (Calibration, error) {
	data, err := evidence.ReadReference(filepath.Dir(store.Path()), evidence.FileReference{Path: ref.ArtifactPath, Digest: ref.Digest})
	if err != nil {
		return Calibration{}, err
	}
	if err := schema.Validate("calibration-v1.schema.json", data); err != nil {
		return Calibration{}, err
	}
	var calibration Calibration
	if err := decode(data, &calibration); err != nil {
		return Calibration{}, err
	}
	if calibration.ID != ref.ID || calibration.Binding.RepositoryID != ref.RepositoryID || calibration.Binding.Epoch != ref.EpochID || calibration.Binding.Snapshot != ref.SnapshotID ||
		calibration.Binding.Evidence.Fingerprint != ref.Fingerprint || calibration.Binding.Evidence.Algorithm != ref.Algorithm || calibration.ContractDigest != ref.ContractDigest || !calibration.Binding.Evidence.ObservedAt.Equal(ref.ObservedAt) {
		return Calibration{}, errors.New("calibration artifact does not match its SQLite reference")
	}
	return calibration, nil
}

func applicability(calibration Calibration, ref storage.CalibrationReference, history storage.History, live repository.EvidenceCapture, config ReportConfig, freshness string) Applicability {
	result := Applicability{Status: "not_applicable", Reasons: []string{}, LanguageDrift: map[string]int{}}
	selected := history.Selected
	if selected == nil {
		result.Reasons = append(result.Reasons, "no-state-observation-before-cutoff")
		return result
	}
	if !ref.ObservedAt.Before(config.Cutoff) {
		result.Reasons = append(result.Reasons, "binding-observed-after-cutoff")
		return result
	}
	if selected.Epoch.ID != ref.EpochID || (selected.Epoch.SupersededAt != nil && selected.Epoch.SupersededAt.Before(config.Cutoff)) {
		result.Reasons = append(result.Reasons, "epoch-mismatch")
		return result
	}
	if config.Producer == nil || *config.Producer != calibration.Producer || calibration.CueContract.SchemaVersion != model.SchemaVersion || calibration.CueContract.SelectionPolicy != evidence.SelectionPolicy ||
		calibration.Runner.BenchmarkVersion != benchmark.Version || calibration.Runner.OutputSchemaVersion != benchmark.AnswerSchemaVersion {
		result.Reasons = append(result.Reasons, "producer-or-cue-contract-incompatible")
		return result
	}
	profile := evidence.ProfileOf(model.Scan{Files: selected.Files})
	result.TrackedFileDrift = profile.TrackedFiles - calibration.Profile.TrackedFiles
	result.TrackedByteDrift = profile.TrackedBytes - calibration.Profile.TrackedBytes
	for name, count := range profile.Languages {
		result.LanguageDrift[name] = count - calibration.Profile.Languages[name]
	}
	for name, count := range calibration.Profile.Languages {
		if _, found := profile.Languages[name]; !found {
			result.LanguageDrift[name] = -count
		}
	}
	if config.Historical {
		if selected.Snapshot.ID == ref.SnapshotID && selected.Snapshot.RepositoryDigest == calibration.Binding.Evidence.TrackedDigest && selected.Snapshot.Basis.StatusDigest == calibration.Binding.Evidence.StatusDigest {
			result.Status = "measured_reference"
			result.Reasons = append(result.Reasons, "retained-binding-observation; not historical publication evidence")
		} else {
			result.Reasons = append(result.Reasons, "historical-content-evidence-unavailable")
		}
		return result
	}
	if freshness == "refresh-needed" || selected.Snapshot.ID != history.Current.Snapshot.ID {
		result.Reasons = append(result.Reasons, "refresh-needed")
		return result
	}
	if selected.Snapshot.ID == ref.SnapshotID {
		if !sameCapture(calibration.Binding.Evidence, live.Evidence) {
			result.Reasons = append(result.Reasons, "same-snapshot-content-evidence-mismatch; renewed calibration required")
			return result
		}
		result.Status = "measured_reference"
	} else {
		result.Status = "estimated"
		result.Reasons = append(result.Reasons, "compatible later snapshot; reference quality not renewed")
	}
	return result
}

func quality(pairs []Pair) Quality {
	result := Quality{Status: "not_observed", DirectFactPassCounts: map[string]int{}, BasicFactPassCounts: map[string]int{}, Qualitative: map[string]Statistic{}, Limitations: []string{"Git facts do not establish semantic completeness", "scores describe calibration responses only; never the current live snapshot"}}
	direct, basic, differences := []float64{}, []float64{}, []float64{}
	for _, pair := range pairs {
		if pair.Direct.Correctness.Ratio != nil {
			direct = append(direct, *pair.Direct.Correctness.Ratio)
			for fact, pass := range pair.Direct.Correctness.Facts {
				if pass {
					result.DirectFactPassCounts[fact]++
				} else if _, exists := result.DirectFactPassCounts[fact]; !exists {
					result.DirectFactPassCounts[fact] = 0
				}
			}
		}
		if pair.Basic.Correctness.Ratio != nil {
			basic = append(basic, *pair.Basic.Correctness.Ratio)
			for fact, pass := range pair.Basic.Correctness.Facts {
				if pass {
					result.BasicFactPassCounts[fact]++
				} else if _, exists := result.BasicFactPassCounts[fact]; !exists {
					result.BasicFactPassCounts[fact] = 0
				}
			}
		}
		if pair.Direct.Correctness.Ratio != nil && pair.Basic.Correctness.Ratio != nil {
			differences = append(differences, *pair.Basic.Correctness.Ratio-*pair.Direct.Correctness.Ratio)
		}
	}
	result.DirectCorrectness = summarize(direct, len(pairs)-len(direct), "derived", "ratio", "dated direct Git-fact correctness")
	result.BasicCorrectness = summarize(basic, len(pairs)-len(basic), "derived", "ratio", "dated basic Git-fact correctness")
	result.CorrectnessDifference = summarize(differences, len(pairs)-len(differences), "derived", "ratio", "paired basic minus direct Git-fact correctness")
	for _, dimension := range []string{"purpose", "architecture", "documentation", "recent_changes", "structural_completeness", "uncertainty_handling"} {
		values := []float64{}
		for _, pair := range pairs {
			d, dok := pair.Direct.Assessment.Scores[dimension]
			a, aok := pair.Basic.Assessment.Scores[dimension]
			if dok && aok {
				values = append(values, float64(a-d))
			}
		}
		result.Qualitative[dimension] = summarize(values, len(pairs)-len(values), "derived", "score difference", "paired basic minus direct; assessor values unchanged")
		if len(values) > 0 {
			result.Status = "derived"
		}
	}
	return result
}

func unavailableMeasurements() map[string]Measurement {
	result := map[string]Measurement{}
	for name, unit := range map[string]string{"cpu_time_ms": "ms", "peak_memory_bytes": "bytes", "filesystem_read_bytes": "bytes", "filesystem_write_bytes": "bytes", "filesystem_io_savings_bytes": "bytes", "freshness_lag_ms": "ms", "task_ready_latency_ms": "ms", "historical_database_size_bytes": "bytes", "runtime_cue_duration_trend_ms": "ms"} {
		result[name] = missing(unit, "operational observation", "no retained comparable observation path")
	}
	return result
}

func timeMeanings() map[string]string {
	return map[string]string{"state": "latest snapshot observation strictly before cutoff; sequence breaks ties", "activity": "metric-row recording time in [start, cutoff), not publication or agent receipt", "binding": "evidence acquisition strictly before cutoff, not import completion", "maintenance": "all recorded epoch baseline/refresh rows before cutoff, including pre-window rows", "live": "current validation and DB size are separate current observations", "reproducibility": "requires the same DB/evidence read basis and cutoff; later retained observations can change a repeated historical result"}
}

// Write uses the same complete result for both formats. The Markdown data
// appendix retains every value, classification, and provenance field verbatim.
func Write(writer io.Writer, report Report, format string) error {
	serialized, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := schema.Validate("savings-report-v1.schema.json", serialized); err != nil {
		return err
	}
	if format == "json" {
		_, err = fmt.Fprintln(writer, string(serialized))
		return err
	}
	if format != "markdown" {
		return errors.New("report format must be json or markdown")
	}
	var output strings.Builder
	fmt.Fprintf(&output, "# RepoCue Quantitative Report\n\nCutoff: %s. Freshness: %s. Calibration: %s.\n\nThese are projected costs, not observed consumer savings.\n", report.Window.Cutoff.Format(time.RFC3339Nano), report.Freshness, report.Applicability.Status)
	if len(report.Projections) > 0 {
		output.WriteString("\n| Metric | Consumers | Median Saving | Unit | Complete Pairs |\n| --- | --- | --- | --- | --- |\n")
		for _, projection := range report.Projections {
			if projection.Metric != "total_tokens" && projection.Metric != "execution_duration_ms" && projection.Metric != "command_executions" {
				continue
			}
			value := "not_observed"
			if projection.Saving.Median != nil {
				value = fmt.Sprintf("%.6g", *projection.Saving.Median)
			}
			fmt.Fprintf(&output, "| %s | %d | %s | %s | %d |\n", projection.Metric, projection.Consumers, value, projection.Saving.Unit, projection.Saving.Count)
		}
	}
	output.WriteString("\n## Complete Measurements And Provenance\n\n```json\n")
	output.Write(serialized)
	output.WriteString("\n```\n")
	_, err = io.WriteString(writer, output.String())
	return err
}
