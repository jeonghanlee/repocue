package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/internal/app"
	_ "modernc.org/sqlite"
)

func TestRefreshGitAndFilesystemTransitions(t *testing.T) {
	tests := []struct {
		name           string
		prepare        func(*testing.T, string)
		mutate         func(*testing.T, string)
		wantChanged    bool
		wantOperations []string
		verify         func(*testing.T, map[string]any, []map[string]any)
	}{
		{
			name: "modified",
			mutate: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "README.md"), "# Example\n\nModified.\n")
			},
			wantChanged: true, wantOperations: []string{"file.content_changed", "repository.working_tree_changed"},
		},
		{
			name: "content and mode changed",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, "README.md")
				writeFile(t, path, "# Example\n\nModified and executable.\n")
				if err := os.Chmod(path, 0o755); err != nil {
					t.Fatal(err)
				}
			},
			wantChanged: true, wantOperations: []string{"file.content_changed", "repository.working_tree_changed"},
			verify: func(t *testing.T, _ map[string]any, changes []map[string]any) {
				for _, change := range changes {
					if change["path"] != "README.md" {
						continue
					}
					before := change["before"].(map[string]any)
					after := change["after"].(map[string]any)
					if before["working_tree_mode"] != "100644" || after["working_tree_mode"] != "100755" {
						t.Fatalf("content delta omitted mode change: before=%#v after=%#v", before, after)
					}
					return
				}
				t.Fatal("content and mode delta was not found")
			},
		},
		{
			name: "staged",
			mutate: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "README.md"), "# Example\n\nStaged.\n")
				runGit(t, root, "add", "README.md")
			},
			wantChanged: true, wantOperations: []string{"file.content_changed", "repository.working_tree_changed"},
			verify: func(t *testing.T, refresh map[string]any, _ []map[string]any) {
				basis := refresh["basis"].(map[string]any)
				if !containsJSONPath(basis["staged"], "README.md") {
					t.Fatalf("staged basis omitted README.md: %#v", basis)
				}
			},
		},
		{
			name: "untracked",
			mutate: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "notes.txt"), "untracked\n")
			},
			wantChanged: true, wantOperations: []string{"repository.working_tree_changed"},
			verify: func(t *testing.T, refresh map[string]any, _ []map[string]any) {
				basis := refresh["basis"].(map[string]any)
				if !containsJSONPath(basis["untracked"], "notes.txt") {
					t.Fatalf("untracked basis omitted notes.txt: %#v", basis)
				}
			},
		},
		{
			name: "renamed",
			mutate: func(t *testing.T, root string) {
				runGit(t, root, "mv", "README.md", "README-renamed.md")
			},
			wantChanged:    true,
			wantOperations: []string{"file.added", "file.deleted", "repository.working_tree_changed"},
		},
		{
			name: "deleted",
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
					t.Fatal(err)
				}
			},
			wantChanged: true, wantOperations: []string{"file.deleted", "repository.working_tree_changed"},
		},
		{
			name: "ignored",
			prepare: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, ".gitignore"), "ignored.tmp\n")
				runGit(t, root, "add", ".gitignore")
				runGit(t, root, "commit", "-m", "Add ignore rule")
			},
			mutate: func(t *testing.T, root string) {
				writeFile(t, filepath.Join(root, "ignored.tmp"), "ignored\n")
			},
			wantChanged: false,
		},
		{
			name: "binary",
			mutate: func(t *testing.T, root string) {
				path := filepath.Join(root, "asset.bin")
				if err := os.WriteFile(path, []byte{0, 1, 2, 3}, 0o644); err != nil {
					t.Fatal(err)
				}
				runGit(t, root, "add", "asset.bin")
			},
			wantChanged: true, wantOperations: []string{"file.added", "repository.working_tree_changed"},
			verify: func(t *testing.T, _ map[string]any, changes []map[string]any) {
				for _, change := range changes {
					if change["path"] != "asset.bin" {
						continue
					}
					after := change["after"].(map[string]any)
					if after["file_type"] != "binary" {
						t.Fatalf("binary file was classified as %#v", after["file_type"])
					}
					return
				}
				t.Fatal("binary delta was not found")
			},
		},
		{
			name: "branch change",
			mutate: func(t *testing.T, root string) {
				runGit(t, root, "switch", "-c", "feature")
			},
			wantChanged: true, wantOperations: []string{"repository.branch_changed"},
		},
		{
			name: "detached HEAD",
			mutate: func(t *testing.T, root string) {
				runGit(t, root, "checkout", "--detach")
			},
			wantChanged: true, wantOperations: []string{"repository.branch_changed"},
			verify: func(t *testing.T, refresh map[string]any, _ []map[string]any) {
				basis := refresh["basis"].(map[string]any)
				if basis["branch"] != nil {
					t.Fatalf("detached HEAD reported branch %#v", basis["branch"])
				}
			},
		},
		{
			name:        "no-op",
			mutate:      func(*testing.T, string) {},
			wantChanged: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := createGitRepository(t)
			if test.prepare != nil {
				test.prepare(t, repository)
			}
			cache := t.TempDir()
			runCLI(t, "init", "--cache-dir", cache, repository)
			test.mutate(t, repository)
			refresh := runCLI(t, "refresh", "--cache-dir", cache, "--repository", repository)
			if refresh["changed"] != test.wantChanged {
				t.Fatalf("changed = %#v, want %v", refresh["changed"], test.wantChanged)
			}
			changes := []map[string]any{}
			if test.wantChanged {
				delta := runCLI(t, "cue", "--cache-dir", cache, "--repository", repository,
					"--since", "snapshot-000001", "--max-tokens", "5000")
				changes = cueChanges(t, delta)
				operations := make([]string, 0, len(changes))
				for _, change := range changes {
					operations = append(operations, change["op"].(string))
				}
				for _, operation := range test.wantOperations {
					if !slices.Contains(operations, operation) {
						t.Fatalf("delta operations %v omit %s", operations, operation)
					}
				}
			} else if refresh["snapshot"] != "snapshot-000001" {
				t.Fatalf("no-op refresh advanced snapshot: %#v", refresh)
			}
			if test.verify != nil {
				test.verify(t, refresh, changes)
			}
		})
	}
}

func TestDirtyRebaselineCreatesCoherentEpoch(t *testing.T) {
	repository := createGitRepository(t)
	cache := t.TempDir()
	runCLI(t, "init", "--cache-dir", cache, repository)
	writeFile(t, filepath.Join(repository, "README.md"), "# Example\n\nDirty.\n")
	rebaseline := runCLI(t, "rebaseline", "--cache-dir", cache, "--repository", repository,
		"--label", "milestone:dirty")
	if rebaseline["epoch"] != "epoch-000002" || rebaseline["superseded_epoch"] != "epoch-000001" {
		t.Fatalf("unexpected rebaseline output: %#v", rebaseline)
	}
	if rebaseline["freshness"] != "dirty-but-indexed" {
		t.Fatalf("unexpected dirty rebaseline freshness: %#v", rebaseline["freshness"])
	}
	status := runCLI(t, "status", "--cache-dir", cache, "--repository", repository)
	if status["freshness"] != "dirty-but-indexed" || status["epoch_count"].(float64) != 2 {
		t.Fatalf("unexpected status after dirty rebaseline: %#v", status)
	}
}

func TestFailedRefreshDoesNotExposePartialSnapshot(t *testing.T) {
	repository := createGitRepository(t)
	cache := t.TempDir()
	runCLI(t, "init", "--cache-dir", cache, repository)
	status := runCLI(t, "status", "--cache-dir", cache, "--repository", repository)
	databasePath := status["database"].(string)
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`CREATE TRIGGER force_refresh_failure BEFORE INSERT ON snapshot_files
		WHEN NEW.path = 'README.md' BEGIN SELECT RAISE(ABORT, 'forced refresh failure'); END`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repository, "README.md"), "# Example\n\nChanged.\n")
	if code, _ := runCLIError(t, context.Background(), "refresh", "--cache-dir", cache, "--repository", repository); code == 0 {
		t.Fatal("refresh unexpectedly succeeded")
	}
	status = runCLI(t, "status", "--cache-dir", cache, "--repository", repository)
	if status["snapshot"] != "snapshot-000001" || status["freshness"] != "refresh-needed" {
		t.Fatalf("failed refresh exposed partial state: %#v", status)
	}
	database, err = sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var snapshots, deltas int
	if err := database.QueryRow("SELECT COUNT(*) FROM snapshots").Scan(&snapshots); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow("SELECT COUNT(*) FROM deltas").Scan(&deltas); err != nil {
		t.Fatal(err)
	}
	if snapshots != 1 || deltas != 0 {
		t.Fatalf("failed refresh retained partial rows: snapshots=%d deltas=%d", snapshots, deltas)
	}
}

func TestCanceledRefreshDoesNotAdvanceSnapshot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the test observes the real scanner file descriptor through procfs")
	}
	repository := createGitRepository(t)
	largePath := filepath.Join(repository, "large.bin")
	large, err := os.Create(largePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := large.Truncate(32 << 20); err != nil {
		large.Close()
		t.Fatal(err)
	}
	if err := large.Close(); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "large.bin")
	runGit(t, repository, "commit", "-m", "Add large fixture")
	cache := t.TempDir()
	runCLI(t, "init", "--cache-dir", cache, repository)
	large, err = os.OpenFile(largePath, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := large.WriteAt([]byte{1}, 0); err != nil {
		large.Close()
		t.Fatal(err)
	}
	if err := large.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	interrupted := make(chan error, 1)
	go cancelWhenFileOpen(largePath, cancel, interrupted)
	if code, _ := runCLIError(t, ctx, "refresh", "--cache-dir", cache, "--repository", repository); code == 0 {
		t.Fatal("canceled refresh unexpectedly succeeded")
	}
	if err := <-interrupted; err != nil {
		t.Fatal(err)
	}
	status := runCLI(t, "status", "--cache-dir", cache, "--repository", repository)
	if status["snapshot"] != "snapshot-000001" || status["freshness"] != "refresh-needed" {
		t.Fatalf("canceled refresh advanced state: %#v", status)
	}
}

func cancelWhenFileOpen(watchedPath string, cancel context.CancelFunc, result chan<- error) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir("/proc/self/fd")
		if err != nil {
			result <- err
			return
		}
		for _, entry := range entries {
			target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
			if err != nil {
				continue
			}
			if target == watchedPath {
				cancel()
				result <- nil
				return
			}
		}
		runtime.Gosched()
	}
	result <- errors.New("refresh did not open the watched file before the deadline")
}

func cueChanges(t *testing.T, cue map[string]any) []map[string]any {
	t.Helper()
	content := cue["content"].(map[string]any)
	rawChanges := content["changes"].([]any)
	changes := make([]map[string]any, 0, len(rawChanges))
	for _, raw := range rawChanges {
		changes = append(changes, raw.(map[string]any))
	}
	return changes
}

func containsJSONPath(value any, path string) bool {
	for _, raw := range value.([]any) {
		if raw == path {
			return true
		}
	}
	return false
}

func runCLIError(t *testing.T, ctx context.Context, args ...string) (int, map[string]any) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := app.Run(ctx, args, &stdout, &stderr)
	var output map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &output); err != nil {
		t.Fatalf("decode error output %q: %v", stderr.String(), err)
	}
	return code, output
}
