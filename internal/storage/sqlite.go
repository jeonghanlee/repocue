package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jeonghanlee/repocue/internal/metrics"
	"github.com/jeonghanlee/repocue/internal/model"
	_ "modernc.org/sqlite"
)

const schemaVersion = 2

var (
	ErrAlreadyInitialized = errors.New("RepoCue is already initialized")
	ErrNotInitialized     = errors.New("RepoCue is not initialized")
	ErrConcurrentUpdate   = errors.New("RepoCue state changed during the operation")
)

type Store struct {
	db   *sql.DB
	path string
}

type Transition struct {
	Epoch             model.Epoch
	Snapshot          model.Snapshot
	DeltaID           string
	Changed           bool
	Run               OperationRun
	DatabaseSizeBytes int64
}

type OperationRun struct {
	ID               string    `json:"id"`
	Operation        string    `json:"operation"`
	EpochID          string    `json:"epoch"`
	SnapshotID       string    `json:"snapshot"`
	CreatedAt        time.Time `json:"created_at"`
	DurationMS       float64   `json:"duration_ms"`
	ScanDurationMS   float64   `json:"scan_duration_ms"`
	GitCommands      int       `json:"git_commands"`
	FilesScanned     int       `json:"files_scanned"`
	BytesScanned     int64     `json:"bytes_scanned"`
	ChangedFileCount int       `json:"changed_file_count"`
}

type CueRun struct {
	ID              string    `json:"id"`
	View            string    `json:"view"`
	SnapshotID      string    `json:"snapshot"`
	SinceSnapshotID *string   `json:"since_snapshot,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	MaxTokens       int       `json:"max_tokens"`
	OutputBytes     int       `json:"output_bytes"`
	EstimatedTokens int       `json:"estimated_tokens"`
}

type Report struct {
	DatabaseSizeBytes int64          `json:"database_size_bytes"`
	Operations        []OperationRun `json:"operations"`
	Cues              []CueRun       `json:"cues"`
}

func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, path: path}
	if err := store.configure(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.ensureSchema(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Initialize(ctx context.Context, repository model.Repository, scan model.Scan) (Transition, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Transition{}, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM repositories").Scan(&count); err != nil {
		return Transition{}, err
	}
	if count != 0 {
		return Transition{}, ErrAlreadyInitialized
	}
	now := scan.Basis.ObservedAt
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO repositories (id, name, root, git_dir, created_at) VALUES (?, ?, ?, ?, ?)",
		repository.ID, repository.Name, repository.Root, repository.GitDir, formatTime(repository.CreatedAt)); err != nil {
		return Transition{}, err
	}
	epoch, err := createEpoch(ctx, tx, repository.ID, "initial", "init", now)
	if err != nil {
		return Transition{}, err
	}
	snapshot, err := createSnapshot(ctx, tx, epoch.ID, nil, "baseline", scan)
	if err != nil {
		return Transition{}, err
	}
	if err := insertFiles(ctx, tx, snapshot.ID, scan.Files); err != nil {
		return Transition{}, err
	}
	run, err := insertOperationRun(ctx, tx, "baseline", epoch.ID, snapshot.ID, scan.Metrics, len(scan.Files))
	if err != nil {
		return Transition{}, err
	}
	if err := tx.Commit(); err != nil {
		return Transition{}, err
	}
	size, err := metrics.DatabaseSize(s.path)
	if err != nil {
		return Transition{}, err
	}
	return Transition{Epoch: epoch, Snapshot: snapshot, Changed: true, Run: run, DatabaseSizeBytes: size}, nil
}

func (s *Store) Refresh(ctx context.Context, expected model.CurrentState, scan model.Scan, items []model.DeltaItem) (Transition, error) {
	changed := expected.Snapshot.RepositoryDigest != scan.RepositoryDigest ||
		expected.Snapshot.Basis.StatusDigest != scan.Basis.StatusDigest
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Transition{}, err
	}
	defer tx.Rollback()
	if err := verifyCurrentSnapshot(ctx, tx, expected.Repository.ID, expected.Snapshot.ID); err != nil {
		return Transition{}, err
	}
	if !changed {
		run, err := insertOperationRun(ctx, tx, "refresh", expected.Epoch.ID, expected.Snapshot.ID, scan.Metrics, 0)
		if err != nil {
			return Transition{}, err
		}
		if err := tx.Commit(); err != nil {
			return Transition{}, err
		}
		size, err := metrics.DatabaseSize(s.path)
		if err != nil {
			return Transition{}, err
		}
		return Transition{Epoch: expected.Epoch, Snapshot: expected.Snapshot, Changed: false, Run: run, DatabaseSizeBytes: size}, nil
	}
	parent := expected.Snapshot.ID
	snapshot, err := createSnapshot(ctx, tx, expected.Epoch.ID, &parent, "refresh", scan)
	if err != nil {
		return Transition{}, err
	}
	if err := insertFiles(ctx, tx, snapshot.ID, scan.Files); err != nil {
		return Transition{}, err
	}
	deltaID, err := insertDelta(ctx, tx, expected.Epoch.ID, expected.Snapshot.ID, snapshot.ID, items, scan.Basis.ObservedAt)
	if err != nil {
		return Transition{}, err
	}
	run, err := insertOperationRun(ctx, tx, "refresh", expected.Epoch.ID, snapshot.ID, scan.Metrics, changedFileCount(items))
	if err != nil {
		return Transition{}, err
	}
	if err := tx.Commit(); err != nil {
		return Transition{}, err
	}
	size, err := metrics.DatabaseSize(s.path)
	if err != nil {
		return Transition{}, err
	}
	return Transition{Epoch: expected.Epoch, Snapshot: snapshot, DeltaID: deltaID, Changed: true, Run: run, DatabaseSizeBytes: size}, nil
}

func (s *Store) Rebaseline(ctx context.Context, expected model.CurrentState, scan model.Scan, label, reason string) (Transition, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Transition{}, err
	}
	defer tx.Rollback()
	if err := verifyCurrentSnapshot(ctx, tx, expected.Repository.ID, expected.Snapshot.ID); err != nil {
		return Transition{}, err
	}
	now := scan.Basis.ObservedAt
	if _, err := tx.ExecContext(ctx,
		"UPDATE epochs SET status = 'superseded', superseded_at = ? WHERE id = ? AND status = 'active'",
		formatTime(now), expected.Epoch.ID); err != nil {
		return Transition{}, err
	}
	epoch, err := createEpoch(ctx, tx, expected.Repository.ID, label, reason, now)
	if err != nil {
		return Transition{}, err
	}
	snapshot, err := createSnapshot(ctx, tx, epoch.ID, nil, "baseline", scan)
	if err != nil {
		return Transition{}, err
	}
	if err := insertFiles(ctx, tx, snapshot.ID, scan.Files); err != nil {
		return Transition{}, err
	}
	run, err := insertOperationRun(ctx, tx, "rebaseline", epoch.ID, snapshot.ID, scan.Metrics, len(scan.Files))
	if err != nil {
		return Transition{}, err
	}
	if err := tx.Commit(); err != nil {
		return Transition{}, err
	}
	size, err := metrics.DatabaseSize(s.path)
	if err != nil {
		return Transition{}, err
	}
	return Transition{Epoch: epoch, Snapshot: snapshot, Changed: true, Run: run, DatabaseSizeBytes: size}, nil
}

func (s *Store) Current(ctx context.Context, repositoryID string) (model.CurrentState, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.CurrentState{}, err
	}
	defer tx.Rollback()
	state, err := currentState(ctx, tx, repositoryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.CurrentState{}, ErrNotInitialized
		}
		return model.CurrentState{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.CurrentState{}, err
	}
	return state, nil
}

func (s *Store) StateAt(ctx context.Context, repositoryID, snapshotID string) (model.CurrentState, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return model.CurrentState{}, err
	}
	defer tx.Rollback()
	repository, err := readRepository(ctx, tx, repositoryID)
	if err != nil {
		return model.CurrentState{}, err
	}
	snapshot, err := readSnapshot(ctx, tx, snapshotID)
	if err != nil {
		return model.CurrentState{}, err
	}
	epoch, err := readEpoch(ctx, tx, snapshot.EpochID)
	if err != nil {
		return model.CurrentState{}, err
	}
	files, err := readFiles(ctx, tx, snapshot.ID)
	if err != nil {
		return model.CurrentState{}, err
	}
	epochCount, err := countEpochs(ctx, tx, repositoryID)
	if err != nil {
		return model.CurrentState{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.CurrentState{}, err
	}
	return model.CurrentState{Repository: repository, Epoch: epoch, Snapshot: snapshot, Files: files, EpochCount: epochCount}, nil
}

func (s *Store) RecordCue(ctx context.Context, run CueRun) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	sequence, err := nextSequence(ctx, tx, "cue_runs")
	if err != nil {
		return err
	}
	run.ID = formatID("cue-run", sequence)
	var since any
	if run.SinceSnapshotID != nil {
		since = *run.SinceSnapshotID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cue_runs
		(id, sequence, view, snapshot_id, since_snapshot_id, created_at, max_tokens, output_bytes, estimated_tokens)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, sequence, run.View, run.SnapshotID, since, formatTime(run.CreatedAt), run.MaxTokens, run.OutputBytes, run.EstimatedTokens)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Metrics(ctx context.Context) (Report, error) {
	report := Report{Operations: []OperationRun{}, Cues: []CueRun{}}
	rows, err := s.db.QueryContext(ctx, `SELECT id, operation, epoch_id, snapshot_id, created_at,
		duration_ms, scan_duration_ms, git_commands, files_scanned, bytes_scanned, changed_file_count
		FROM operation_runs ORDER BY sequence`)
	if err != nil {
		return Report{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var run OperationRun
		var created string
		if err := rows.Scan(&run.ID, &run.Operation, &run.EpochID, &run.SnapshotID, &created,
			&run.DurationMS, &run.ScanDurationMS, &run.GitCommands, &run.FilesScanned,
			&run.BytesScanned, &run.ChangedFileCount); err != nil {
			return Report{}, err
		}
		run.CreatedAt, err = parseTime(created)
		if err != nil {
			return Report{}, err
		}
		report.Operations = append(report.Operations, run)
	}
	if err := rows.Err(); err != nil {
		return Report{}, err
	}
	cueRows, err := s.db.QueryContext(ctx, `SELECT id, view, snapshot_id, since_snapshot_id, created_at,
		max_tokens, output_bytes, estimated_tokens FROM cue_runs ORDER BY sequence`)
	if err != nil {
		return Report{}, err
	}
	defer cueRows.Close()
	for cueRows.Next() {
		var run CueRun
		var since sql.NullString
		var created string
		if err := cueRows.Scan(&run.ID, &run.View, &run.SnapshotID, &since, &created,
			&run.MaxTokens, &run.OutputBytes, &run.EstimatedTokens); err != nil {
			return Report{}, err
		}
		if since.Valid {
			run.SinceSnapshotID = &since.String
		}
		run.CreatedAt, err = parseTime(created)
		if err != nil {
			return Report{}, err
		}
		report.Cues = append(report.Cues, run)
	}
	if err := cueRows.Err(); err != nil {
		return Report{}, err
	}
	report.DatabaseSizeBytes, err = metrics.DatabaseSize(s.path)
	return report, err
}

func (s *Store) configure(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to SQLite database: %w", err)
	}
	statements := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = DELETE",
		"PRAGMA synchronous = FULL",
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure SQLite: %w", err)
		}
	}
	return nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version < 0 || version > schemaVersion {
		return fmt.Errorf("unsupported SQLite schema version %d", version)
	}
	if version == schemaVersion {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if version == 0 {
		for _, statement := range schemaStatements() {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("create SQLite schema: %w", err)
			}
		}
	} else if version == 1 {
		for _, statement := range schemaMigrationV2() {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate SQLite schema to version 2: %w", err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}

func createEpoch(ctx context.Context, tx *sql.Tx, repositoryID, label, reason string, now time.Time) (model.Epoch, error) {
	sequence, err := nextSequence(ctx, tx, "epochs")
	if err != nil {
		return model.Epoch{}, err
	}
	epoch := model.Epoch{ID: formatID("epoch", sequence), Sequence: sequence, Label: label, Reason: reason, Status: "active", CreatedAt: now}
	_, err = tx.ExecContext(ctx, `INSERT INTO epochs
		(id, sequence, repository_id, label, reason, status, created_at) VALUES (?, ?, ?, ?, ?, 'active', ?)`,
		epoch.ID, epoch.Sequence, repositoryID, epoch.Label, epoch.Reason, formatTime(epoch.CreatedAt))
	return epoch, err
}

func createSnapshot(ctx context.Context, tx *sql.Tx, epochID string, parent *string, kind string, scan model.Scan) (model.Snapshot, error) {
	sequence, err := nextSequence(ctx, tx, "snapshots")
	if err != nil {
		return model.Snapshot{}, err
	}
	var epochSequence int64
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(epoch_sequence), 0) + 1 FROM snapshots WHERE epoch_id = ?", epochID).Scan(&epochSequence); err != nil {
		return model.Snapshot{}, err
	}
	basisJSON, err := json.Marshal(scan.Basis)
	if err != nil {
		return model.Snapshot{}, err
	}
	var totalBytes int64
	for _, file := range scan.Files {
		if file.Exists {
			totalBytes += file.SizeBytes
		}
	}
	snapshot := model.Snapshot{
		ID: formatID("snapshot", sequence), EpochID: epochID, Sequence: sequence,
		EpochSequence: epochSequence, Kind: kind, ParentSnapshotID: parent,
		Basis: scan.Basis, RepositoryDigest: scan.RepositoryDigest,
		FileCount: len(scan.Files), TotalBytes: totalBytes,
	}
	var parentValue any
	if parent != nil {
		parentValue = *parent
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO snapshots
		(id, sequence, epoch_id, epoch_sequence, kind, parent_snapshot_id, basis_json,
		repository_digest, file_count, total_bytes, observed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.ID, snapshot.Sequence, snapshot.EpochID, snapshot.EpochSequence, snapshot.Kind,
		parentValue, string(basisJSON), snapshot.RepositoryDigest, snapshot.FileCount,
		snapshot.TotalBytes, formatTime(snapshot.Basis.ObservedAt))
	return snapshot, err
}

func insertFiles(ctx context.Context, tx *sql.Tx, snapshotID string, files []model.File) error {
	statement, err := tx.PrepareContext(ctx, `INSERT INTO snapshot_files
		(snapshot_id, entity_id, path, index_mode, index_object, working_tree_mode,
		exists_flag, size_bytes, content_digest, file_type, language, document_flag, working_state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, file := range files {
		if _, err := statement.ExecContext(ctx, snapshotID, file.EntityID, file.Path, file.IndexMode,
			file.IndexObject, file.WorkingTreeMode, boolInt(file.Exists), file.SizeBytes,
			file.ContentDigest, file.FileType, file.Language, boolInt(file.Document), file.WorkingState); err != nil {
			return err
		}
	}
	return nil
}

func insertDelta(ctx context.Context, tx *sql.Tx, epochID, fromID, toID string, items []model.DeltaItem, now time.Time) (string, error) {
	sequence, err := nextSequence(ctx, tx, "deltas")
	if err != nil {
		return "", err
	}
	deltaID := formatID("delta", sequence)
	if _, err := tx.ExecContext(ctx, `INSERT INTO deltas
		(id, sequence, epoch_id, from_snapshot_id, to_snapshot_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		deltaID, sequence, epochID, fromID, toID, formatTime(now)); err != nil {
		return "", err
	}
	statement, err := tx.PrepareContext(ctx, `INSERT INTO delta_items
		(delta_id, ordinal, operation, entity_id, path, before_json, after_json) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return "", err
	}
	defer statement.Close()
	for index, item := range items {
		before, err := nullableJSON(item.Before)
		if err != nil {
			return "", err
		}
		after, err := nullableJSON(item.After)
		if err != nil {
			return "", err
		}
		if _, err := statement.ExecContext(ctx, deltaID, index, item.Operation, item.Entity, item.Path, before, after); err != nil {
			return "", err
		}
	}
	return deltaID, nil
}

func insertOperationRun(ctx context.Context, tx *sql.Tx, operation, epochID, snapshotID string, scan model.ScanMetrics, changed int) (OperationRun, error) {
	sequence, err := nextSequence(ctx, tx, "operation_runs")
	if err != nil {
		return OperationRun{}, err
	}
	run := OperationRun{
		ID: formatID("run", sequence), Operation: operation, EpochID: epochID, SnapshotID: snapshotID,
		CreatedAt: time.Now().UTC(), DurationMS: operationDurationMS(scan),
		ScanDurationMS: float64(scan.Duration) / float64(time.Millisecond), GitCommands: scan.GitCommands,
		FilesScanned: scan.FilesScanned, BytesScanned: scan.BytesScanned, ChangedFileCount: changed,
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO operation_runs
		(id, sequence, operation, epoch_id, snapshot_id, created_at, duration_ms,
		scan_duration_ms, git_commands, files_scanned, bytes_scanned, changed_file_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, sequence, run.Operation, run.EpochID, run.SnapshotID, formatTime(run.CreatedAt),
		run.DurationMS, run.ScanDurationMS, run.GitCommands, run.FilesScanned,
		run.BytesScanned, run.ChangedFileCount)
	return run, err
}

func operationDurationMS(scan model.ScanMetrics) float64 {
	if !scan.StartedAt.IsZero() {
		return float64(time.Since(scan.StartedAt)) / float64(time.Millisecond)
	}
	return float64(scan.Duration) / float64(time.Millisecond)
}

func currentState(ctx context.Context, tx *sql.Tx, repositoryID string) (model.CurrentState, error) {
	repository, err := readRepository(ctx, tx, repositoryID)
	if err != nil {
		return model.CurrentState{}, err
	}
	epoch, err := readActiveEpoch(ctx, tx, repositoryID)
	if err != nil {
		return model.CurrentState{}, err
	}
	var snapshotID string
	if err := tx.QueryRowContext(ctx,
		"SELECT id FROM snapshots WHERE epoch_id = ? ORDER BY epoch_sequence DESC LIMIT 1", epoch.ID).Scan(&snapshotID); err != nil {
		return model.CurrentState{}, err
	}
	snapshot, err := readSnapshot(ctx, tx, snapshotID)
	if err != nil {
		return model.CurrentState{}, err
	}
	files, err := readFiles(ctx, tx, snapshotID)
	if err != nil {
		return model.CurrentState{}, err
	}
	epochCount, err := countEpochs(ctx, tx, repositoryID)
	if err != nil {
		return model.CurrentState{}, err
	}
	return model.CurrentState{Repository: repository, Epoch: epoch, Snapshot: snapshot, Files: files, EpochCount: epochCount}, nil
}

func readRepository(ctx context.Context, tx *sql.Tx, id string) (model.Repository, error) {
	var repository model.Repository
	var created string
	err := tx.QueryRowContext(ctx,
		"SELECT id, name, root, git_dir, created_at FROM repositories WHERE id = ?", id).
		Scan(&repository.ID, &repository.Name, &repository.Root, &repository.GitDir, &created)
	if err != nil {
		return model.Repository{}, err
	}
	repository.CreatedAt, err = parseTime(created)
	return repository, err
}

func readActiveEpoch(ctx context.Context, tx *sql.Tx, repositoryID string) (model.Epoch, error) {
	var id string
	if err := tx.QueryRowContext(ctx,
		"SELECT id FROM epochs WHERE repository_id = ? AND status = 'active'", repositoryID).Scan(&id); err != nil {
		return model.Epoch{}, err
	}
	return readEpoch(ctx, tx, id)
}

func readEpoch(ctx context.Context, tx *sql.Tx, id string) (model.Epoch, error) {
	var epoch model.Epoch
	var created string
	var superseded sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT id, sequence, label, reason, status, created_at, superseded_at
		FROM epochs WHERE id = ?`, id).
		Scan(&epoch.ID, &epoch.Sequence, &epoch.Label, &epoch.Reason, &epoch.Status, &created, &superseded)
	if err != nil {
		return model.Epoch{}, err
	}
	epoch.CreatedAt, err = parseTime(created)
	if err != nil {
		return model.Epoch{}, err
	}
	if superseded.Valid {
		value, err := parseTime(superseded.String)
		if err != nil {
			return model.Epoch{}, err
		}
		epoch.SupersededAt = &value
	}
	return epoch, nil
}

func readSnapshot(ctx context.Context, tx *sql.Tx, id string) (model.Snapshot, error) {
	var snapshot model.Snapshot
	var parent sql.NullString
	var basisJSON string
	var observed string
	err := tx.QueryRowContext(ctx, `SELECT id, sequence, epoch_id, epoch_sequence, kind,
		parent_snapshot_id, basis_json, repository_digest, file_count, total_bytes, observed_at
		FROM snapshots WHERE id = ?`, id).
		Scan(&snapshot.ID, &snapshot.Sequence, &snapshot.EpochID, &snapshot.EpochSequence, &snapshot.Kind,
			&parent, &basisJSON, &snapshot.RepositoryDigest, &snapshot.FileCount, &snapshot.TotalBytes, &observed)
	if err != nil {
		return model.Snapshot{}, err
	}
	if parent.Valid {
		snapshot.ParentSnapshotID = &parent.String
	}
	if err := json.Unmarshal([]byte(basisJSON), &snapshot.Basis); err != nil {
		return model.Snapshot{}, err
	}
	snapshot.Basis.ObservedAt, err = parseTime(observed)
	return snapshot, err
}

func readFiles(ctx context.Context, tx *sql.Tx, snapshotID string) ([]model.File, error) {
	rows, err := tx.QueryContext(ctx, `SELECT entity_id, path, index_mode, index_object,
		working_tree_mode, exists_flag, size_bytes, content_digest, file_type, language,
		document_flag, working_state FROM snapshot_files WHERE snapshot_id = ? ORDER BY path`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := []model.File{}
	for rows.Next() {
		var file model.File
		var exists, document int
		if err := rows.Scan(&file.EntityID, &file.Path, &file.IndexMode, &file.IndexObject,
			&file.WorkingTreeMode, &exists, &file.SizeBytes, &file.ContentDigest, &file.FileType,
			&file.Language, &document, &file.WorkingState); err != nil {
			return nil, err
		}
		file.Exists = exists != 0
		file.Document = document != 0
		files = append(files, file)
	}
	return files, rows.Err()
}

func verifyCurrentSnapshot(ctx context.Context, tx *sql.Tx, repositoryID, expected string) error {
	state, err := currentState(ctx, tx, repositoryID)
	if err != nil {
		return err
	}
	if state.Snapshot.ID != expected {
		return ErrConcurrentUpdate
	}
	return nil
}

func countEpochs(ctx context.Context, tx *sql.Tx, repositoryID string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM epochs WHERE repository_id = ?", repositoryID).Scan(&count)
	return count, err
}

func nextSequence(ctx context.Context, tx *sql.Tx, table string) (int64, error) {
	allowed := map[string]bool{"epochs": true, "snapshots": true, "deltas": true, "operation_runs": true, "cue_runs": true}
	if !allowed[table] {
		return 0, fmt.Errorf("unsupported sequence table %q", table)
	}
	var sequence int64
	err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence), 0) + 1 FROM "+table).Scan(&sequence)
	return sequence, err
}

func formatID(prefix string, sequence int64) string {
	return fmt.Sprintf("%s-%06d", prefix, sequence)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableJSON(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	serialized, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(serialized), nil
}

func changedFileCount(items []model.DeltaItem) int {
	count := 0
	for _, item := range items {
		if item.Path != "" {
			count++
		}
	}
	return count
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func schemaStatements() []string {
	return []string{
		`CREATE TABLE repositories (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		root TEXT NOT NULL UNIQUE,
		git_dir TEXT NOT NULL,
		created_at TEXT NOT NULL
	) WITHOUT ROWID`,
		`CREATE TABLE epochs (
		id TEXT PRIMARY KEY,
		sequence INTEGER NOT NULL UNIQUE,
		repository_id TEXT NOT NULL REFERENCES repositories(id),
		label TEXT NOT NULL,
		reason TEXT NOT NULL,
		status TEXT NOT NULL CHECK (status IN ('active', 'superseded')),
		created_at TEXT NOT NULL,
		superseded_at TEXT
	) WITHOUT ROWID`,
		`CREATE UNIQUE INDEX one_active_epoch ON epochs(repository_id) WHERE status = 'active'`,
		`CREATE TABLE snapshots (
		id TEXT PRIMARY KEY,
		sequence INTEGER NOT NULL UNIQUE,
		epoch_id TEXT NOT NULL REFERENCES epochs(id),
		epoch_sequence INTEGER NOT NULL,
		kind TEXT NOT NULL CHECK (kind IN ('baseline', 'refresh')),
		parent_snapshot_id TEXT REFERENCES snapshots(id),
		basis_json TEXT NOT NULL,
		repository_digest TEXT NOT NULL,
		file_count INTEGER NOT NULL,
		total_bytes INTEGER NOT NULL,
		observed_at TEXT NOT NULL,
		UNIQUE(epoch_id, epoch_sequence)
	) WITHOUT ROWID`,
		`CREATE TABLE snapshot_files (
		snapshot_id TEXT NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
		entity_id TEXT NOT NULL,
		path TEXT NOT NULL,
		index_mode TEXT NOT NULL,
		index_object TEXT NOT NULL,
		working_tree_mode TEXT NOT NULL,
		exists_flag INTEGER NOT NULL CHECK (exists_flag IN (0, 1)),
		size_bytes INTEGER NOT NULL,
		content_digest TEXT NOT NULL,
		file_type TEXT NOT NULL,
		language TEXT NOT NULL,
		document_flag INTEGER NOT NULL CHECK (document_flag IN (0, 1)),
		working_state TEXT NOT NULL,
		PRIMARY KEY(snapshot_id, entity_id),
		UNIQUE(snapshot_id, path)
	) WITHOUT ROWID`,
		`CREATE INDEX snapshot_files_path ON snapshot_files(path)`,
		`CREATE TABLE deltas (
		id TEXT PRIMARY KEY,
		sequence INTEGER NOT NULL UNIQUE,
		epoch_id TEXT NOT NULL REFERENCES epochs(id),
		from_snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
		to_snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
		created_at TEXT NOT NULL,
		UNIQUE(from_snapshot_id, to_snapshot_id)
	) WITHOUT ROWID`,
		`CREATE TABLE delta_items (
		delta_id TEXT NOT NULL REFERENCES deltas(id) ON DELETE CASCADE,
		ordinal INTEGER NOT NULL,
		operation TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		path TEXT NOT NULL,
		before_json TEXT,
		after_json TEXT,
		PRIMARY KEY(delta_id, ordinal)
	) WITHOUT ROWID`,
		`CREATE TABLE operation_runs (
		id TEXT PRIMARY KEY,
		sequence INTEGER NOT NULL UNIQUE,
		operation TEXT NOT NULL,
		epoch_id TEXT NOT NULL REFERENCES epochs(id),
		snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
		created_at TEXT NOT NULL,
		duration_ms REAL NOT NULL,
		scan_duration_ms REAL NOT NULL,
		git_commands INTEGER NOT NULL,
		files_scanned INTEGER NOT NULL,
		bytes_scanned INTEGER NOT NULL,
		changed_file_count INTEGER NOT NULL
	) WITHOUT ROWID`,
		`CREATE TABLE cue_runs (
		id TEXT PRIMARY KEY,
		sequence INTEGER NOT NULL UNIQUE,
		view TEXT NOT NULL,
		snapshot_id TEXT NOT NULL REFERENCES snapshots(id),
		since_snapshot_id TEXT REFERENCES snapshots(id),
		created_at TEXT NOT NULL,
		max_tokens INTEGER NOT NULL,
		output_bytes INTEGER NOT NULL,
		estimated_tokens INTEGER NOT NULL
	) WITHOUT ROWID`,
	}
}

func schemaMigrationV2() []string {
	return []string{
		"ALTER TABLE operation_runs ADD COLUMN scan_duration_ms REAL NOT NULL DEFAULT 0",
		"ALTER TABLE operation_runs ADD COLUMN git_commands INTEGER NOT NULL DEFAULT 0",
	}
}
