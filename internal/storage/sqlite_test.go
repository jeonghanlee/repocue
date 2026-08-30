package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/internal/model"
)

func TestRefreshRollbackPreservesCurrentSnapshot(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, t.TempDir()+"/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	repository := model.Repository{ID: "repo-test", Name: "test", Root: "/tmp/test", GitDir: "/tmp/test/.git", CreatedAt: now}
	initial := model.Scan{
		Basis:            model.Basis{StatusDigest: "sha256:one", WorkingTreeDigest: "sha256:one", ObservedAt: now},
		Files:            []model.File{{EntityID: "file:a", Path: "a", Exists: true, ContentDigest: "sha256:one"}},
		RepositoryDigest: "sha256:one",
	}
	transition, err := store.Initialize(ctx, repository, initial)
	if err != nil {
		t.Fatal(err)
	}
	current, err := store.Current(ctx, repository.ID)
	if err != nil {
		t.Fatal(err)
	}
	bad := initial
	bad.Basis.StatusDigest = "sha256:two"
	bad.RepositoryDigest = "sha256:two"
	bad.Files = append(bad.Files, bad.Files[0])
	if _, err := store.Refresh(ctx, current, bad, nil); err == nil {
		t.Fatal("refresh unexpectedly succeeded")
	}
	after, err := store.Current(ctx, repository.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Snapshot.ID != transition.Snapshot.ID {
		t.Fatalf("current snapshot changed from %s to %s", transition.Snapshot.ID, after.Snapshot.ID)
	}
}

func TestOpenMigratesSchemaVersionOne(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`CREATE TABLE operation_runs (
		id TEXT PRIMARY KEY,
		sequence INTEGER NOT NULL UNIQUE,
		operation TEXT NOT NULL,
		epoch_id TEXT NOT NULL,
		snapshot_id TEXT NOT NULL,
		created_at TEXT NOT NULL,
		duration_ms REAL NOT NULL,
		files_scanned INTEGER NOT NULL,
		bytes_scanned INTEGER NOT NULL,
		changed_file_count INTEGER NOT NULL
	) WITHOUT ROWID`)
	if err != nil {
		database.Close()
		t.Fatal(err)
	}
	if _, err := database.Exec("PRAGMA user_version = 1"); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err := store.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("schema version = %d, want 2", version)
	}
	columns := map[string]bool{}
	rows, err := store.db.QueryContext(ctx, "PRAGMA table_info(operation_runs)")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var position, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&position, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		columns[name] = true
	}
	if !columns["scan_duration_ms"] || !columns["git_commands"] {
		t.Fatalf("migration columns missing: %#v", columns)
	}
}
