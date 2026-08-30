package model

import "time"

const SchemaVersion = "repocue/1"

type Basis struct {
	Branch            *string   `json:"branch"`
	Head              *string   `json:"head"`
	Dirty             bool      `json:"dirty"`
	Staged            []string  `json:"staged"`
	Unstaged          []string  `json:"unstaged"`
	Untracked         []string  `json:"untracked"`
	IndexDigest       string    `json:"index_digest"`
	StatusDigest      string    `json:"status_digest"`
	WorkingTreeDigest string    `json:"working_tree_digest"`
	ObservedAt        time.Time `json:"observed_at"`
}

func (b Basis) SameState(other Basis) bool {
	return equalStringPointers(b.Branch, other.Branch) &&
		equalStringPointers(b.Head, other.Head) &&
		b.StatusDigest == other.StatusDigest
}

func equalStringPointers(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

type File struct {
	EntityID        string `json:"entity_id"`
	Path            string `json:"path"`
	IndexMode       string `json:"index_mode"`
	IndexObject     string `json:"index_object"`
	WorkingTreeMode string `json:"working_tree_mode"`
	Exists          bool   `json:"exists"`
	SizeBytes       int64  `json:"size_bytes"`
	ContentDigest   string `json:"content_digest"`
	FileType        string `json:"file_type"`
	Language        string `json:"language,omitempty"`
	Document        bool   `json:"document"`
	WorkingState    string `json:"working_state"`
}

type ScanMetrics struct {
	StartedAt    time.Time     `json:"-"`
	Duration     time.Duration `json:"-"`
	GitCommands  int           `json:"git_commands"`
	FilesScanned int           `json:"files_scanned"`
	BytesScanned int64         `json:"bytes_scanned"`
}

type Scan struct {
	Basis            Basis
	Files            []File
	RepositoryDigest string
	Metrics          ScanMetrics
}

type Repository struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Root      string `json:"root"`
	GitDir    string `json:"git_dir"`
	CreatedAt time.Time
}

type Epoch struct {
	ID           string     `json:"id"`
	Sequence     int64      `json:"sequence"`
	Label        string     `json:"label"`
	Reason       string     `json:"reason"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	SupersededAt *time.Time `json:"superseded_at,omitempty"`
}

type Snapshot struct {
	ID               string  `json:"id"`
	EpochID          string  `json:"epoch_id"`
	Sequence         int64   `json:"sequence"`
	EpochSequence    int64   `json:"epoch_sequence"`
	Kind             string  `json:"kind"`
	ParentSnapshotID *string `json:"parent_snapshot_id,omitempty"`
	Basis            Basis   `json:"basis"`
	RepositoryDigest string  `json:"repository_digest"`
	FileCount        int     `json:"file_count"`
	TotalBytes       int64   `json:"total_bytes"`
}

type CurrentState struct {
	Repository Repository
	Epoch      Epoch
	Snapshot   Snapshot
	Files      []File
	EpochCount int
}

type DeltaItem struct {
	Operation string `json:"op"`
	Entity    string `json:"entity"`
	Path      string `json:"path,omitempty"`
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
}
