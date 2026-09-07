package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocationCacheReusesOnlyVerifiedRoot(t *testing.T) {
	root, cache := createScanFixture(t), t.TempDir()
	cold := openLocationWithCalls(t, root, cache, 1)
	warm := openLocationWithCalls(t, root, cache, 0)
	if *cold != *warm {
		t.Fatalf("cached repository differs: cold=%#v warm=%#v", cold, warm)
	}
	writeScanFixture(t, root, "README.md", "working-tree edit\n")
	openLocationWithCalls(t, root, cache, 0)
	scan, err := warm.FullScan(context.Background())
	if err != nil || !scan.Basis.Dirty {
		t.Fatalf("location reuse hid a working-tree edit: dirty=%v err=%v", scan.Basis.Dirty, err)
	}
	if _, err := os.Stat(filepath.Join(cache, cold.ID, "state.db")); !os.IsNotExist(err) {
		t.Fatalf("location discovery created canonical state: %v", err)
	}
}

func TestLocationCacheInvalidation(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*testing.T, string, string)
	}{
		{name: "missing", change: func(t *testing.T, _ string, hint string) {
			if err := os.Rename(hint, hint+".saved"); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "corrupt", change: func(t *testing.T, _ string, hint string) {
			writeScanFixture(t, filepath.Dir(hint), filepath.Base(hint), "{invalid")
		}},
		{name: "unsupported version", change: func(t *testing.T, _ string, hint string) {
			data, err := os.ReadFile(hint)
			if err != nil {
				t.Fatal(err)
			}
			writeScanFixture(t, filepath.Dir(hint), filepath.Base(hint), strings.Replace(string(data), `"version":1`, `"version":999`, 1))
		}},
		{name: "oversized hint", change: func(t *testing.T, _ string, hint string) {
			writeScanFixture(t, filepath.Dir(hint), filepath.Base(hint), strings.Repeat("x", locationReadLimit+1))
		}},
		{name: "configuration change", change: func(t *testing.T, root, _ string) {
			runFixtureGit(t, root, "config", "core.filemode", "false")
		}},
		{name: "branch change", change: func(t *testing.T, root, _ string) {
			runFixtureGit(t, root, "switch", "-c", "new-branch")
		}},
		{name: "Git directory recreated", change: func(t *testing.T, root, _ string) {
			if err := os.Rename(filepath.Join(root, ".git"), filepath.Join(t.TempDir(), "saved.git")); err != nil {
				t.Fatal(err)
			}
			runFixtureGit(t, root, "init", "-b", "replacement")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, cache := createScanFixture(t), t.TempDir()
			repo := openLocationWithCalls(t, root, cache, 1)
			openLocationWithCalls(t, root, cache, 0)
			test.change(t, root, filepath.Join(cache, repo.ID, locationFile))
			discovered := openLocationWithCalls(t, root, cache, 1)
			openLocationWithCalls(t, root, cache, 0)
			if *repo != *discovered {
				t.Fatalf("same-root discovery changed repository identity: %#v %#v", repo, discovered)
			}
		})
	}
}

func TestLocationCacheBypassesNonstandardDiscovery(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*testing.T, string) string
	}{
		{name: "subdirectory", change: func(t *testing.T, root string) string {
			path := filepath.Join(root, "nested")
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{name: "linked worktree", change: func(t *testing.T, root string) string {
			linked := filepath.Join(t.TempDir(), "linked")
			runFixtureGit(t, root, "worktree", "add", "-b", "linked", linked)
			return linked
		}},
		{name: "separate Git directory", change: func(t *testing.T, root string) string {
			runFixtureGit(t, root, "init", "--separate-git-dir", filepath.Join(t.TempDir(), "separate.git"))
			return root
		}},
		{name: "explicit worktree configuration", change: func(t *testing.T, root string) string {
			runFixtureGit(t, root, "config", "core.worktree", root)
			return root
		}},
		{name: "configuration include", change: func(t *testing.T, root string) string {
			included := filepath.Join(t.TempDir(), "included.config")
			writeScanFixture(t, filepath.Dir(included), filepath.Base(included), "[core]\n\tfilemode = false\n")
			runFixtureGit(t, root, "config", "include.path", included)
			return root
		}},
		{name: "Git environment override", change: func(t *testing.T, root string) string {
			t.Setenv("GIT_WORK_TREE", root)
			return root
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, cache := createScanFixture(t), t.TempDir()
			openLocationWithCalls(t, root, cache, 1)
			path := test.change(t, root)
			first := openLocationWithCalls(t, path, cache, 1)
			second := openLocationWithCalls(t, path, cache, 1)
			if *first != *second {
				t.Fatalf("repeated discovery differs: %#v %#v", first, second)
			}
		})
	}
}

func TestLocationCacheDoesNotSurviveRepositoryMove(t *testing.T) {
	root, cache := createScanFixture(t), t.TempDir()
	before := openLocationWithCalls(t, root, cache, 1)
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	after := openLocationWithCalls(t, moved, cache, 1)
	if before.ID == after.ID {
		t.Fatal("moving a repository reused the old root identity")
	}
	openLocationWithCalls(t, moved, cache, 0)
	if repo, _, err := OpenCached(context.Background(), root, cache); err == nil || repo != nil {
		t.Fatalf("missing old root was accepted: repo=%#v error=%v", repo, err)
	}
}

func TestLocationCacheWriteFailureDoesNotBlockDiscovery(t *testing.T) {
	root, cache := createScanFixture(t), t.TempDir()
	repo := openLocationWithCalls(t, root, cache, 1)
	hint := filepath.Join(cache, repo.ID, locationFile)
	if err := os.Rename(hint, hint+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(hint, 0o700); err != nil {
		t.Fatal(err)
	}
	result, warning, err := OpenCached(context.Background(), root, cache)
	if err != nil || warning == nil || result == nil || *result != *repo {
		t.Fatalf("discovery was not preserved: repo=%#v warning=%v error=%v", result, warning, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, _, err := OpenCached(canceled, root, cache); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation returned repo=%#v error=%v", result, err)
	}
}

func TestLocationCacheConcurrentPublication(t *testing.T) {
	root, cache := createScanFixture(t), t.TempDir()
	results := make(chan error, 8)
	for range cap(results) {
		go func() {
			repo, warning, err := OpenCached(context.Background(), root, cache)
			if err != nil || warning != nil || repo == nil || repo.Root != root {
				results <- fmt.Errorf("concurrent discovery: repo=%#v warning=%v error=%v", repo, warning, err)
				return
			}
			results <- nil
		}()
	}
	for range cap(results) {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	repo := openLocationWithCalls(t, root, cache, 0)
	path := filepath.Join(cache, repo.ID, locationFile)
	if _, ok := readLocation(path); !ok {
		t.Fatal("concurrent publication left an invalid location hint")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("location hint is not private: %v", err)
	}
	leftovers, err := filepath.Glob(filepath.Join(cache, repo.ID, ".location-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary location files remain: %v error=%v", leftovers, err)
	}
}

func openLocationWithCalls(t *testing.T, root, cache string, want int) *Repository {
	t.Helper()
	tracePath := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", tracePath)
	repo, warning, err := OpenCached(context.Background(), root, cache)
	if err != nil || warning != nil {
		t.Fatalf("open cached location: error=%v warning=%v", err, warning)
	}
	trace, err := os.ReadFile(tracePath)
	if err != nil && !(want == 0 && os.IsNotExist(err)) {
		t.Fatal(err)
	}
	if got := strings.Count(string(trace), "built-in: git "); got != want {
		t.Fatalf("location lookup used %d Git calls, want %d: %s", got, want, trace)
	}
	actual, err := Open(context.Background(), root)
	if err != nil || *actual != *repo {
		t.Fatalf("cached location differs from real Git discovery: got=%#v actual=%#v error=%v", repo, actual, err)
	}
	return repo
}
