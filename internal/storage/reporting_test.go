package storage

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/internal/evidence"
	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
)

func reportingStore(t *testing.T) (*Store, *repository.Repository, model.CurrentState) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	if output, err := exec.Command("git", "-C", root, "init", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("Git init: %v %s", err, output)
	}
	repo, err := repository.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := repo.FullScan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err := store.Initialize(ctx, model.Repository{ID: repo.ID, Root: root, GitDir: repo.GitDir, Name: repo.Name, CreatedAt: scan.Basis.ObservedAt}, scan); err != nil {
		t.Fatal(err)
	}
	state, err := store.Current(ctx, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	return store, repo, state
}

func testReference(state model.CurrentState) (CalibrationReference, []byte) {
	content := []byte(`{"numeric_evidence":1}`)
	return CalibrationReference{ID: "calibration-" + strings.TrimPrefix(evidence.Digest(content), "sha256:"), RepositoryID: state.Repository.ID, EpochID: state.Epoch.ID,
		SnapshotID: state.Snapshot.ID, ObservedAt: time.Now().UTC(), Fingerprint: evidence.Digest(content), Algorithm: repository.EvidenceAlgorithm,
		ContractDigest: evidence.Digest(content), Model: "fixture", Benchmark: "fixture"}, content
}

func TestCalibrationPublicationFailuresAndReopen(t *testing.T) {
	for _, mode := range []string{"before publication", "after publication", "cancelled", "existing wrong digest", "symlink directory"} {
		t.Run(mode, func(t *testing.T) {
			store, repo, state := reportingStore(t)
			ref, content := testReference(state)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			directory := filepath.Join(filepath.Dir(store.Path()), "calibrations")
			outside := t.TempDir()
			final := filepath.Join(directory, ref.ID+".json")
			switch mode {
			case "symlink directory":
				if err := os.Symlink(outside, directory); err != nil {
					t.Fatal(err)
				}
			case "before publication":
				if err := os.WriteFile(directory, []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "after publication":
				// A real deferred foreign-key violation fails COMMIT only after
				// the shipped publication code has created the immutable file.
				if _, err := store.db.Exec(`CREATE TABLE commit_guard (id TEXT REFERENCES repositories(id) DEFERRABLE INITIALLY DEFERRED);
				CREATE TRIGGER fail_commit AFTER INSERT ON calibration_references BEGIN INSERT INTO commit_guard VALUES ('missing-repository'); END`); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				cancel()
			case "existing wrong digest":
				if err := os.Mkdir(directory, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(final, []byte("wrong"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := store.PublishCalibration(ctx, state, ref, content); err == nil {
				t.Fatal("failure was not propagated")
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 0 {
				t.Fatal("publication followed cache symlink outside")
			}
			if mode == "after publication" {
				data, err := os.ReadFile(final)
				if err != nil || string(data) != string(content) {
					t.Fatalf("post-publication failure path not exercised: %s %v", data, err)
				}
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := OpenReadOnly(context.Background(), store.Path())
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			history, err := reopened.ReadHistory(context.Background(), repo.ID, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			if len(history.Calibrations) != 0 || history.Current.Snapshot.ID != state.Snapshot.ID {
				t.Fatal("failed transaction exposed partial current state")
			}
		})
	}
}

func TestCalibrationPublicationRechecksCurrentStateAndDigest(t *testing.T) {
	store, repo, state := reportingStore(t)
	ctx := context.Background()
	ref, content := testReference(state)
	first, err := store.PublishCalibration(ctx, state, ref, content)
	if err != nil {
		t.Fatal(err)
	}
	ref.ObservedAt = ref.ObservedAt.Add(time.Hour)
	second, err := store.PublishCalibration(ctx, state, ref, content)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("non-idempotent publication: %v", err)
	}
	scan, err := repo.FullScan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Rebaseline(ctx, state, scan, "new epoch", "manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PublishCalibration(ctx, state, ref, content); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale expected state accepted: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(store.Path()), first.ArtifactPath), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := evidence.ReadReference(filepath.Dir(store.Path()), evidence.FileReference{Path: first.ArtifactPath, Digest: first.Digest}); err == nil {
		t.Fatal("changed immutable file was trusted")
	}
}

func TestVersionTwoReadOnlyAndVersionOnePopulatedMigration(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(string(rune('0'+version)), func(t *testing.T) {
			store, repo, state := reportingStore(t)
			before, err := store.Metrics(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			// The shipped initializer populates the real historical tables;
			// removing only later columns restores their versioned SQL shape.
			if _, err := store.db.Exec("DROP TABLE calibration_references"); err != nil {
				t.Fatal(err)
			}
			if version == 1 {
				if _, err := store.db.Exec("ALTER TABLE operation_runs DROP COLUMN scan_duration_ms; ALTER TABLE operation_runs DROP COLUMN git_commands; PRAGMA user_version=1"); err != nil {
					t.Fatal(err)
				}
			} else if _, err := store.db.Exec("PRAGMA user_version=2"); err != nil {
				t.Fatal(err)
			}
			path := store.Path()
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			if version == 2 {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				ro, err := OpenReadOnly(context.Background(), path)
				if err != nil {
					t.Fatal(err)
				}
				history, err := ro.ReadHistory(context.Background(), repo.ID, time.Now().UTC())
				if err != nil {
					t.Fatal(err)
				}
				if history.SchemaVersion != 2 || history.Current.Snapshot.ID != state.Snapshot.ID {
					t.Fatal("read-only changed schema or state")
				}
				if _, err := ro.db.Exec("PRAGMA user_version=3"); err == nil {
					t.Fatal("read-only database allowed mutation")
				}
				if err := ro.Close(); err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(path)
				if err != nil || string(data) != string(after) {
					t.Fatal("read-only open changed DB bytes")
				}
			}
			migrated, err := Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			defer migrated.Close()
			history, err := migrated.ReadHistory(context.Background(), repo.ID, time.Now().UTC())
			if err != nil {
				t.Fatal(err)
			}
			if history.SchemaVersion != 3 || history.Current.Snapshot.ID != state.Snapshot.ID || len(history.Operations) != len(before.Operations) {
				t.Fatal("migration lost history")
			}
			left, right := before.Operations[0], history.Operations[0]
			if version == 1 {
				left.ScanDurationMS = 0
				left.GitCommands = 0
			}
			if !reflect.DeepEqual(left, right) {
				t.Fatalf("migration rewrote legacy values: %+v %+v", left, right)
			}
		})
	}
}

func TestHistoryReadersRemainCoherentDuringPublicationAndRefresh(t *testing.T) {
	store, repo, state := reportingStore(t)
	ctx := context.Background()
	scan, err := repo.IncrementalScan(ctx, state.Files)
	if err != nil {
		t.Fatal(err)
	}
	ro, err := OpenReadOnly(ctx, store.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	ref, content := testReference(state)
	var workers sync.WaitGroup
	issues := make(chan error, 2)
	workers.Add(1)
	go func() {
		defer workers.Done()
		for index := 0; index < 12; index++ {
			if _, err := store.Refresh(ctx, state, scan, nil); err != nil {
				issues <- err
				return
			}
			if _, err := store.PublishCalibration(ctx, state, ref, content); err != nil {
				issues <- err
				return
			}
		}
	}()
	for index := 0; index < 12; index++ {
		history, err := ro.ReadHistory(ctx, repo.ID, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if history.Current.Snapshot.ID != state.Snapshot.ID || len(history.Calibrations) > 1 {
			t.Fatal("incoherent history")
		}
		for _, ref := range history.Calibrations {
			if ref.SnapshotID != history.Current.Snapshot.ID {
				t.Fatal("reference does not identify complete snapshot")
			}
			if _, err := evidence.ReadReference(filepath.Dir(store.Path()), evidence.FileReference{Path: ref.ArtifactPath, Digest: ref.Digest}); err != nil {
				t.Fatal(err)
			}
		}
	}
	workers.Wait()
	close(issues)
	for err := range issues {
		t.Fatal(err)
	}
}

func TestHistoryObservationCutoffsAndSequenceTies(t *testing.T) {
	store, repo, first := reportingStore(t)
	ctx := context.Background()
	scan, err := repo.FullScan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Rebaseline(ctx, first, scan, "second", "manual"); err != nil {
		t.Fatal(err)
	}
	second, err := store.Current(ctx, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	instant := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
	// Timestamp columns are the outer clock boundary. The shipped storage
	// readers still decode real SQLite state and apply the selection rules.
	if _, err := store.db.Exec("UPDATE snapshots SET observed_at = ?", instant.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("UPDATE operation_runs SET created_at = ?", instant.Add(time.Second).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	at, err := store.ReadHistory(ctx, repo.ID, instant)
	if err != nil {
		t.Fatal(err)
	}
	if at.Selected != nil {
		t.Fatal("strict cutoff included equal observation time")
	}
	after, err := store.ReadHistory(ctx, repo.ID, instant.Add(time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	if after.Selected == nil || after.Selected.Snapshot.ID != second.Snapshot.ID || after.Selected.Snapshot.Sequence <= first.Snapshot.Sequence {
		t.Fatal("observation tie did not choose greatest sequence")
	}
	for _, run := range after.Operations {
		if !run.CreatedAt.After(after.Selected.Snapshot.Basis.ObservedAt) {
			t.Fatal("recording time replaced observation time")
		}
	}
}
