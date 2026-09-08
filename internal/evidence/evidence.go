package evidence

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
)

const (
	SchemaVersion   = "repocue/calibration-evidence-1"
	SelectionPolicy = "repocue/basic-overview-1"
)

type Producer struct {
	Revision         string `json:"revision"`
	ExecutableDigest string `json:"executable_digest"`
	Modified         bool   `json:"modified"`
	ModuleVersion    string `json:"module_version"`
}

type CueContract struct {
	View            string `json:"view"`
	SchemaVersion   string `json:"schema_version"`
	MaxTokens       int    `json:"max_tokens"`
	SelectionPolicy string `json:"selection_policy"`
}

type Profile struct {
	TrackedFiles int            `json:"tracked_files"`
	TrackedBytes int64          `json:"tracked_bytes"`
	Languages    map[string]int `json:"languages"`
}

type FileReference struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type SourceReport struct {
	FileReference
	PairID           string                     `json:"pair_id"`
	Condition        string                     `json:"condition"`
	RunIndex         int                        `json:"run_index"`
	Snapshot         string                     `json:"snapshot"`
	StateFingerprint string                     `json:"state_fingerprint"`
	Before           repository.ContentEvidence `json:"before"`
	After            repository.ContentEvidence `json:"after"`
}

type Scoring struct {
	ReportDigest   string      `json:"report_digest"`
	Basis          model.Basis `json:"basis"`
	ScorerRevision string      `json:"scorer_revision"`
	ScoredAt       time.Time   `json:"scored_at"`
	Passed         *int        `json:"passed,omitempty"`
	Total          *int        `json:"total,omitempty"`
}

type CueCLI struct {
	PairID            string    `json:"pair_id"`
	ExecutableDigest  string    `json:"executable_digest"`
	Arguments         []string  `json:"arguments"`
	CacheState        string    `json:"cache_state"`
	LocationHintState string    `json:"location_hint_state"`
	StartedAt         time.Time `json:"started_at"`
	CompletedAt       time.Time `json:"completed_at"`
	ExitStatus        int       `json:"exit_status"`
	OutputDigest      string    `json:"output_digest"`
	OutputBytes       int       `json:"output_bytes"`
	EstimatedTokens   int       `json:"estimated_tokens"`
	Snapshot          string    `json:"snapshot"`
	Freshness         string    `json:"freshness"`
	DurationMS        float64   `json:"duration_ms"`
	GitInvocations    *int      `json:"git_invocations"`
	GitTraceDigest    string    `json:"git_trace_digest,omitempty"`
	GitStatus         string    `json:"git_status"`
}

type AssessmentReference struct {
	FileReference
	ReportDigest string     `json:"report_digest"`
	ResponseID   string     `json:"response_id"`
	AssessedAt   *time.Time `json:"assessed_at"`
}

// Document is retained externally with raw reports. Consumers persist selected
// numeric fields and provenance, not this document's scoring basis or paths.
type Document struct {
	SchemaVersion string                     `json:"schema_version"`
	Producer      Producer                   `json:"producer"`
	Cue           CueContract                `json:"cue"`
	Profile       Profile                    `json:"profile"`
	Capture       repository.ContentEvidence `json:"capture"`
	Reports       []SourceReport             `json:"reports"`
	Scoring       []Scoring                  `json:"scoring"`
	CueCLI        []CueCLI                   `json:"cue_cli"`
	Assessments   []AssessmentReference      `json:"assessments"`
	Records       []FileReference            `json:"records"`
	Artifacts     map[string][]byte          `json:"-"`
}

func ProfileOf(scan model.Scan) Profile {
	profile := Profile{TrackedFiles: len(scan.Files), Languages: map[string]int{}}
	for _, file := range scan.Files {
		profile.TrackedBytes += file.SizeBytes
		language := file.Language
		if language == "" {
			language = "unknown"
		}
		profile.Languages[language]++
	}
	return profile
}

func Digest(value []byte) string {
	hash := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func Identity(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return Digest(encoded), nil
}

func ExecutableProducer(path string) (Producer, error) {
	file, err := os.Open(path)
	if err != nil {
		return Producer{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return Producer{}, err
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return Producer{}, fmt.Errorf("read producer build information: %w", err)
	}
	producer := Producer{ExecutableDigest: "sha256:" + hex.EncodeToString(hash.Sum(nil)), ModuleVersion: info.Main.Version}
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			producer.Revision = setting.Value
		case "vcs.modified":
			producer.Modified = setting.Value == "true"
		}
	}
	if producer.Revision == "" {
		return Producer{}, fmt.Errorf("producer executable has no vcs.revision provenance")
	}
	return producer, nil
}

// ReadReference resolves only contained regular files and verifies the exact
// retained bytes. A digest is identity evidence, not a signature of its author.
func ReadReference(directory string, ref FileReference) ([]byte, error) {
	if !filepath.IsLocal(ref.Path) || filepath.IsAbs(ref.Path) || strings.Contains(ref.Path, "\\") {
		return nil, fmt.Errorf("evidence reference must be a local relative path")
	}
	for _, component := range strings.Split(ref.Path, "/") {
		if component == ".." {
			return nil, fmt.Errorf("evidence reference contains parent traversal")
		}
	}
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	path, err := filepath.EvalSymlinks(filepath.Join(root, ref.Path))
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(relative) {
		return nil, fmt.Errorf("evidence reference escapes declared directory")
	}
	contained, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer contained.Close()
	file, err := contained.Open(relative)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("evidence reference is not a readable regular file")
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if Digest(data) != ref.Digest {
		return nil, fmt.Errorf("evidence digest mismatch")
	}
	return data, nil
}
