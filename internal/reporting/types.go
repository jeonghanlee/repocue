package reporting

import (
	"time"

	"github.com/jeonghanlee/repocue/internal/evaluation"
	"github.com/jeonghanlee/repocue/internal/evidence"
	"github.com/jeonghanlee/repocue/internal/repository"
)

const (
	CalibrationSchema = "repocue/calibration-1"
	ReportSchema      = "repocue/savings-report-1"
)

type Measurement struct {
	Status      string   `json:"status"`
	Value       *float64 `json:"value"`
	Unit        string   `json:"unit"`
	Basis       string   `json:"basis"`
	Limitations []string `json:"limitations"`
}

type Statistic struct {
	Status   string    `json:"status"`
	Unit     string    `json:"unit"`
	Basis    string    `json:"basis"`
	Values   []float64 `json:"values"`
	Count    int       `json:"count"`
	Unknown  int       `json:"unknown"`
	Median   *float64  `json:"median"`
	Minimum  *float64  `json:"minimum"`
	Maximum  *float64  `json:"maximum"`
	Total    *float64  `json:"total"`
	Coverage string    `json:"coverage"`
}

type Correctness struct {
	Status         string          `json:"status"`
	Passed         *int            `json:"passed"`
	Total          *int            `json:"total"`
	Ratio          *float64        `json:"ratio"`
	Facts          map[string]bool `json:"facts"`
	ScorerRevision string          `json:"scorer_revision"`
	ScoredAt       *time.Time      `json:"scored_at"`
}

type Assessment struct {
	Status     string         `json:"status"`
	ResponseID string         `json:"response_id"`
	Assessor   string         `json:"assessor"`
	Rubric     string         `json:"rubric"`
	Digest     string         `json:"digest"`
	Scores     map[string]int `json:"scores"`
	AssessedAt *time.Time     `json:"assessed_at"`
	TimeStatus string         `json:"time_status"`
}

type Sample struct {
	ReportDigest     string                 `json:"report_digest"`
	Condition        string                 `json:"condition"`
	RunIndex         int                    `json:"run_index"`
	SourceSnapshot   string                 `json:"source_snapshot"`
	Head             *string                `json:"head"`
	StateFingerprint string                 `json:"state_fingerprint"`
	ObservedAt       time.Time              `json:"observed_at"`
	Metrics          map[string]Measurement `json:"metrics"`
	Correctness      Correctness            `json:"correctness"`
	Assessment       Assessment             `json:"assessment"`
}

type Pair struct {
	ID            string                 `json:"id"`
	MaintenanceID string                 `json:"maintenance_id"`
	Direct        Sample                 `json:"direct"`
	Basic         Sample                 `json:"basic"`
	Cue           map[string]Measurement `json:"cue"`
	CueObservedAt *time.Time             `json:"cue_observed_at"`
	Baseline      map[string]Measurement `json:"baseline"`
	Update        map[string]Measurement `json:"update"`
}

type Binding struct {
	RepositoryID string                     `json:"repository_id"`
	Epoch        string                     `json:"epoch"`
	Snapshot     string                     `json:"snapshot"`
	Evidence     repository.ContentEvidence `json:"evidence"`
}

type Calibration struct {
	SchemaVersion  string                    `json:"schema_version"`
	ID             string                    `json:"id"`
	Binding        Binding                   `json:"binding"`
	EvidenceDigest string                    `json:"evidence_digest"`
	SourceDigests  []string                  `json:"source_digests"`
	Producer       evidence.Producer         `json:"producer"`
	CueContract    evidence.CueContract      `json:"cue_contract"`
	Profile        evidence.Profile          `json:"profile"`
	Runner         evaluation.RunnerMetadata `json:"runner"`
	ContractDigest string                    `json:"contract_digest"`
	SampleGrade    string                    `json:"sample_grade"`
	Pairs          []Pair                    `json:"pairs"`
	Comparisons    map[string]Comparison     `json:"comparisons"`
	Limitations    []string                  `json:"limitations"`
}

type PairDifference struct {
	PairID     string      `json:"pair_id"`
	Direct     Measurement `json:"direct"`
	Basic      Measurement `json:"basic"`
	Difference Measurement `json:"difference"`
}

type Comparison struct {
	Pairs         []PairDifference `json:"pairs"`
	Direct        Statistic        `json:"direct"`
	Basic         Statistic        `json:"basic"`
	Difference    Statistic        `json:"difference"`
	PositivePairs int              `json:"positive_pairs"`
}

type PairProjection struct {
	PairID    string      `json:"pair_id"`
	Direct    Measurement `json:"direct"`
	Assisted  Measurement `json:"assisted"`
	Saving    Measurement `json:"saving"`
	Percent   Measurement `json:"percent"`
	BreakEven Measurement `json:"break_even"`
}

type Projection struct {
	Consumers       int64            `json:"consumers"`
	Explicit        bool             `json:"explicit"`
	Metric          string           `json:"metric"`
	Maintenance     Measurement      `json:"maintenance"`
	Pairs           []PairProjection `json:"pairs"`
	Direct          Statistic        `json:"direct"`
	Assisted        Statistic        `json:"assisted"`
	Saving          Statistic        `json:"saving"`
	Percent         Statistic        `json:"percent"`
	FiniteBreakEven Statistic        `json:"finite_break_even"`
}
