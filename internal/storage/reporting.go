package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/jeonghanlee/repocue/internal/evidence"
	"github.com/jeonghanlee/repocue/internal/model"
)

const calibrationSchema = `CREATE TABLE calibration_references (
	id TEXT PRIMARY KEY,
	repository_id TEXT NOT NULL REFERENCES repositories(id),
	epoch_id TEXT NOT NULL REFERENCES epochs(id),
	snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
	artifact_path TEXT NOT NULL UNIQUE,
	digest TEXT NOT NULL,
	observed_at TEXT NOT NULL,
	fingerprint TEXT NOT NULL,
	algorithm TEXT NOT NULL,
	contract_digest TEXT NOT NULL,
	model TEXT NOT NULL,
	benchmark TEXT NOT NULL
) WITHOUT ROWID`

type CalibrationReference struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	EpochID        string    `json:"epoch"`
	SnapshotID     string    `json:"snapshot"`
	ArtifactPath   string    `json:"artifact_path"`
	Digest         string    `json:"digest"`
	ObservedAt     time.Time `json:"observed_at"`
	Fingerprint    string    `json:"fingerprint"`
	Algorithm      string    `json:"algorithm"`
	ContractDigest string    `json:"contract_digest"`
	Model          string    `json:"model"`
	Benchmark      string    `json:"benchmark"`
}

type History struct {
	SchemaVersion int
	Current       model.CurrentState
	Selected      *model.CurrentState
	Epochs        []model.Epoch
	Snapshots     []model.Snapshot
	Operations    []OperationRun
	Cues          []CueRun
	Calibrations  []CalibrationReference
}

// OpenReadOnly neither creates a cache nor changes database schema/journaling.
func OpenReadOnly(ctx context.Context, path string) (*Store, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}
	parameters := url.Values{"mode": {"ro"}, "_pragma": {"busy_timeout(5000)", "query_only(1)"}}
	u.RawQuery = parameters.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, fmt.Errorf("open read-only SQLite: %w", err)
	}
	if version != 2 && version != 3 {
		db.Close()
		return nil, fmt.Errorf("unsupported read-only SQLite schema version %d", version)
	}
	return &Store{db: db, path: absolute}, nil
}

// ReadHistory uses one transaction for state, metrics, and references. Selection
// describes observation before cutoff, never a reconstructed publication time.
func (s *Store) ReadHistory(ctx context.Context, repositoryID string, cutoff time.Time) (History, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return History{}, err
	}
	defer tx.Rollback()
	history := History{Epochs: []model.Epoch{}, Snapshots: []model.Snapshot{}, Calibrations: []CalibrationReference{}}
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&history.SchemaVersion); err != nil {
		return History{}, err
	}
	if history.SchemaVersion != 2 && history.SchemaVersion != 3 {
		return History{}, fmt.Errorf("unsupported reporting schema")
	}
	history.Current, err = currentState(ctx, tx, repositoryID)
	if err != nil {
		return History{}, err
	}
	epochIDs, err := historyIDs(ctx, tx, "SELECT id FROM epochs WHERE repository_id = ? ORDER BY sequence", repositoryID)
	if err != nil {
		return History{}, err
	}
	for _, id := range epochIDs {
		epoch, err := readEpoch(ctx, tx, id)
		if err != nil {
			return History{}, err
		}
		history.Epochs = append(history.Epochs, epoch)
	}
	snapshotIDs, err := historyIDs(ctx, tx, `SELECT snapshots.id FROM snapshots JOIN epochs ON epochs.id = snapshots.epoch_id
		WHERE epochs.repository_id = ? ORDER BY snapshots.sequence`, repositoryID)
	if err != nil {
		return History{}, err
	}
	for _, id := range snapshotIDs {
		item, err := readSnapshot(ctx, tx, id)
		if err != nil {
			return History{}, err
		}
		history.Snapshots = append(history.Snapshots, item)
		if item.Basis.ObservedAt.Before(cutoff) && (history.Selected == nil ||
			!item.Basis.ObservedAt.Before(history.Selected.Snapshot.Basis.ObservedAt)) {
			history.Selected = &model.CurrentState{Repository: history.Current.Repository, Snapshot: item, EpochCount: len(history.Epochs)}
		}
	}
	if history.Selected != nil {
		history.Selected.Epoch, err = readEpoch(ctx, tx, history.Selected.Snapshot.EpochID)
		if err != nil {
			return History{}, err
		}
		history.Selected.Files, err = readFiles(ctx, tx, history.Selected.Snapshot.ID)
		if err != nil {
			return History{}, err
		}
	}
	measurements, err := readMetrics(ctx, tx)
	if err != nil {
		return History{}, err
	}
	history.Operations, history.Cues = measurements.Operations, measurements.Cues
	if history.SchemaVersion == 3 {
		rows, err := tx.QueryContext(ctx, `SELECT id, repository_id, epoch_id, snapshot_id, artifact_path,
			digest, observed_at, fingerprint, algorithm, contract_digest, model, benchmark
			FROM calibration_references WHERE repository_id = ? ORDER BY id`, repositoryID)
		if err != nil {
			return History{}, err
		}
		for rows.Next() {
			ref, err := readCalibrationReference(rows)
			if err != nil {
				rows.Close()
				return History{}, err
			}
			history.Calibrations = append(history.Calibrations, ref)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return History{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return History{}, err
	}
	return history, nil
}

func historyIDs(ctx context.Context, tx *sql.Tx, query, repositoryID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, repositoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

type referenceScanner interface{ Scan(...any) error }

func readCalibrationReference(row referenceScanner) (CalibrationReference, error) {
	var ref CalibrationReference
	var observed string
	if err := row.Scan(&ref.ID, &ref.RepositoryID, &ref.EpochID, &ref.SnapshotID, &ref.ArtifactPath,
		&ref.Digest, &observed, &ref.Fingerprint, &ref.Algorithm, &ref.ContractDigest, &ref.Model, &ref.Benchmark); err != nil {
		return CalibrationReference{}, err
	}
	var err error
	ref.ObservedAt, err = parseTime(observed)
	return ref, err
}

// PublishCalibration serializes publishers through SQLite before immutable file
// publication. Failed transactions may leave unreferenced files, never trusted
// partial references. Reimport returns the original reference and observation.
func (s *Store) PublishCalibration(ctx context.Context, expected model.CurrentState, ref CalibrationReference, content []byte) (CalibrationReference, error) {
	if !regexp.MustCompile(`^calibration-[a-f0-9]{64}$`).MatchString(ref.ID) ||
		ref.RepositoryID != expected.Repository.ID || ref.EpochID != expected.Epoch.ID || ref.SnapshotID != expected.Snapshot.ID {
		return CalibrationReference{}, fmt.Errorf("invalid calibration binding")
	}
	ref.ArtifactPath = filepath.ToSlash(filepath.Join("calibrations", ref.ID+".json"))
	ref.Digest = evidence.Digest(content)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return CalibrationReference{}, err
	}
	defer tx.Rollback()
	current, err := currentState(ctx, tx, expected.Repository.ID)
	if err != nil {
		return CalibrationReference{}, err
	}
	if current.Snapshot.ID != expected.Snapshot.ID || current.Epoch.ID != expected.Epoch.ID {
		return CalibrationReference{}, ErrConcurrentUpdate
	}
	existing, err := readCalibrationReference(tx.QueryRowContext(ctx, `SELECT id, repository_id, epoch_id, snapshot_id, artifact_path,
		digest, observed_at, fingerprint, algorithm, contract_digest, model, benchmark FROM calibration_references WHERE id = ?`, ref.ID))
	if err == nil {
		if existing.Fingerprint != ref.Fingerprint || existing.ContractDigest != ref.ContractDigest || existing.EpochID != ref.EpochID {
			return CalibrationReference{}, fmt.Errorf("calibration identity collision")
		}
		if _, err := evidence.ReadReference(filepath.Dir(s.path), evidence.FileReference{Path: existing.ArtifactPath, Digest: existing.Digest}); err != nil {
			return CalibrationReference{}, err
		}
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return CalibrationReference{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO calibration_references
		(id, repository_id, epoch_id, snapshot_id, artifact_path, digest, observed_at, fingerprint, algorithm, contract_digest, model, benchmark)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, ref.ID, ref.RepositoryID, ref.EpochID, ref.SnapshotID, ref.ArtifactPath,
		ref.Digest, formatTime(ref.ObservedAt), ref.Fingerprint, ref.Algorithm, ref.ContractDigest, ref.Model, ref.Benchmark); err != nil {
		return CalibrationReference{}, err
	}
	directory := filepath.Join(filepath.Dir(s.path), "calibrations")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return CalibrationReference{}, err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return CalibrationReference{}, fmt.Errorf("calibration directory must be a local directory")
	}
	file, err := os.CreateTemp(directory, ".calibration-*")
	if err != nil {
		return CalibrationReference{}, err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(content); err != nil {
		file.Close()
		return CalibrationReference{}, err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return CalibrationReference{}, err
	}
	if err := file.Close(); err != nil {
		return CalibrationReference{}, err
	}
	final := filepath.Join(filepath.Dir(s.path), filepath.FromSlash(ref.ArtifactPath))
	if err := os.Link(file.Name(), final); err != nil && !errors.Is(err, os.ErrExist) {
		return CalibrationReference{}, err
	}
	if _, err := evidence.ReadReference(filepath.Dir(s.path), evidence.FileReference{Path: ref.ArtifactPath, Digest: ref.Digest}); err != nil {
		return CalibrationReference{}, err
	}
	if err := tx.Commit(); err != nil {
		return CalibrationReference{}, err
	}
	return ref, nil
}
