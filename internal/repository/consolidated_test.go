package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Compare the real scanner with independent Git queries on the same repository.
// Git is not replaced by a test double on either side of the comparison.
func TestConsolidatedBasisMatchesGitQueries(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*testing.T, string)
	}{
		{name: "clean"},
		{name: "modified", change: func(t *testing.T, root string) {
			writeScanFixture(t, root, "README.md", "modified\n")
		}},
		{name: "staged and unstaged", change: func(t *testing.T, root string) {
			writeScanFixture(t, root, "README.md", "staged\n")
			runFixtureGit(t, root, "add", "README.md")
			writeScanFixture(t, root, "README.md", "unstaged\n")
		}},
		{name: "staged rename with status configuration", change: func(t *testing.T, root string) {
			runFixtureGit(t, root, "config", "status.renames", "copies")
			runFixtureGit(t, root, "config", "color.status", "always")
			runFixtureGit(t, root, "config", "status.relativePaths", "true")
			runFixtureGit(t, root, "mv", "README.md", "moved name.md")
			writeScanFixture(t, root, "moved name.md", "modified rename\n")
		}},
		{name: "unstaged deletion", change: func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "staged deletion and untracked replacement", change: func(t *testing.T, root string) {
			runFixtureGit(t, root, "rm", "README.md")
			writeScanFixture(t, root, "README.md", "replacement\n")
		}},
		{name: "untracked and ignored", change: func(t *testing.T, root string) {
			writeScanFixture(t, root, "notes/nested file.txt", "untracked\n")
			writeScanFixture(t, root, "ignored.tmp", "ignored\n")
			runFixtureGit(t, root, "config", "status.showUntrackedFiles", "no")
		}},
		{name: "unusual paths", change: func(t *testing.T, root string) {
			for _, path := range []string{"a\ttab.md", "line\nbreak.md", "-option.md", " spaced .md", "caf\u00e9.md"} {
				writeScanFixture(t, root, path, "staged\n")
				runFixtureGit(t, root, "add", "--", path)
				writeScanFixture(t, root, path, "unstaged\n")
			}
		}},
		{name: "binary", change: func(t *testing.T, root string) {
			writeScanFixture(t, root, "asset.bin", "\x00\x01\x02")
			runFixtureGit(t, root, "add", "asset.bin")
		}},
		{name: "intent to add", change: func(t *testing.T, root string) {
			writeScanFixture(t, root, "planned.txt", "not staged\n")
			runFixtureGit(t, root, "add", "--intent-to-add", "planned.txt")
		}},
		{name: "detached HEAD", change: func(t *testing.T, root string) {
			runFixtureGit(t, root, "checkout", "--detach")
		}},
		{name: "branch change", change: func(t *testing.T, root string) {
			runFixtureGit(t, root, "switch", "-c", "feature/topic")
		}},
		{name: "branch named detached", change: func(t *testing.T, root string) {
			runFixtureGit(t, root, "switch", "-c", "(detached)")
		}},
		{name: "branch and tag share a name", change: func(t *testing.T, root string) {
			runFixtureGit(t, root, "tag", "main")
		}},
		{name: "packed branch and tag share a name", change: func(t *testing.T, root string) {
			runFixtureGit(t, root, "tag", "main")
			runFixtureGit(t, root, "pack-refs", "--all")
		}},
		{name: "ambiguous references with warnings disabled", change: func(t *testing.T, root string) {
			runFixtureGit(t, root, "tag", "main")
			runFixtureGit(t, root, "config", "core.warnAmbiguousRefs", "false")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := createScanFixture(t)
			if test.change != nil {
				test.change(t, root)
			}
			assertBasisMatchesGit(t, root)
		})
	}
}

func TestConsolidatedBasisWithoutCommit(t *testing.T) {
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	writeScanFixture(t, root, "first.md", "initial\n")
	runFixtureGit(t, root, "add", "first.md")
	assertBasisMatchesGit(t, root)
}

func TestConsolidatedBasisWithSubmodule(t *testing.T) {
	for _, state := range []string{"clean", "untracked", "modified", "commit", "ignore none", "ignore all"} {
		t.Run(state, func(t *testing.T) {
			root, source := createScanFixture(t), createScanFixture(t)
			runFixtureGit(t, root, "-c", "protocol.file.allow=always", "submodule", "add", source, "module")
			runFixtureGit(t, root, "commit", "-m", "Add submodule")
			module := filepath.Join(root, "module")
			switch state {
			case "untracked", "ignore none":
				writeScanFixture(t, module, "untracked.txt", "new\n")
				if state == "ignore none" {
					runFixtureGit(t, root, "config", "diff.ignoreSubmodules", "none")
				}
			case "modified", "commit", "ignore all":
				writeScanFixture(t, module, "README.md", "modified\n")
				if state == "commit" {
					runFixtureGit(t, module, "add", "README.md")
					runFixtureGit(t, module, "-c", "user.name=RepoCue Test", "-c", "user.email=repocue@example.invalid", "commit", "-m", "Change submodule")
				}
				if state == "ignore all" {
					runFixtureGit(t, root, "config", "diff.ignoreSubmodules", "all")
				}
			}
			assertBasisMatchesGit(t, root)
		})
	}
}

func TestConsolidatedIndexRejectsHiddenAndUnmergedEntries(t *testing.T) {
	for _, state := range []string{"assume-unchanged", "skip-worktree", "unmerged"} {
		t.Run(state, func(t *testing.T) {
			root := createScanFixture(t)
			repo, err := Open(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			previous, err := repo.FullScan(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if state == "unmerged" {
				runFixtureGit(t, root, "switch", "-c", "other")
				writeScanFixture(t, root, "README.md", "other\n")
				runFixtureGit(t, root, "commit", "-am", "Change other branch")
				runFixtureGit(t, root, "switch", "main")
				writeScanFixture(t, root, "README.md", "main\n")
				runFixtureGit(t, root, "commit", "-am", "Change main branch")
				if _, err := exec.Command("git", "-C", root, "merge", "other").CombinedOutput(); err == nil {
					t.Fatal("expected a real merge conflict")
				}
			} else {
				runFixtureGit(t, root, "update-index", "--"+state, "README.md")
			}
			for _, full := range []bool{true, false} {
				if full {
					_, err = repo.FullScan(context.Background())
				} else {
					_, err = repo.IncrementalScan(context.Background(), previous.Files)
				}
				if err == nil || (state == "unmerged" && !strings.Contains(err.Error(), "unmerged")) ||
					(state != "unmerged" && !strings.Contains(err.Error(), state)) {
					t.Fatalf("full=%v: unexpected rejection for %s: %v", full, state, err)
				}
			}
		})
	}
}

func TestConsolidatedDiscovery(t *testing.T) {
	root := createScanFixture(t)
	subdir := filepath.Join(root, "nested", "directory")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	runFixtureGit(t, root, "worktree", "add", "-b", "linked", linked)
	paths := []string{root, subdir, linked}
	if runtime.GOOS != "windows" {
		alias := filepath.Join(t.TempDir(), "alias")
		if err := os.Symlink(root, alias); err != nil {
			t.Fatal(err)
		}
		newline := filepath.Join(t.TempDir(), "line\nbreak")
		if err := os.MkdirAll(newline, 0o755); err != nil {
			t.Fatal(err)
		}
		runFixtureGit(t, newline, "init", "-b", "main")
		paths = append(paths, alias, newline)
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			tracePath := filepath.Join(t.TempDir(), "git.trace")
			t.Setenv("GIT_TRACE", tracePath)
			repo, err := Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			trace, err := os.ReadFile(tracePath)
			if err != nil {
				t.Fatal(err)
			}
			wantCalls := 1
			if strings.Contains(path, "\n") {
				wantCalls = 3
			}
			if calls := strings.Count(string(trace), "built-in: git "); calls != wantCalls {
				t.Fatalf("discovery calls=%d, want %d", calls, wantCalls)
			}
			wantRoot := strings.TrimSuffix(runFixtureGitOutput(t, path, "rev-parse", "--show-toplevel"), "\n")
			wantRoot, err = filepath.EvalSymlinks(wantRoot)
			if err != nil {
				t.Fatal(err)
			}
			wantDir := strings.TrimSuffix(runFixtureGitOutput(t, wantRoot, "rev-parse", "--absolute-git-dir"), "\n")
			if repo.Root != wantRoot || repo.GitDir != wantDir {
				t.Fatalf("discovery=%#v, want root=%q gitdir=%q", repo, wantRoot, wantDir)
			}
		})
	}
}

func assertBasisMatchesGit(t *testing.T, root string) {
	t.Helper()
	repo, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(repo.GitDir, "index")
	beforeIndex, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := repo.FullScan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	afterIndex, err := os.ReadFile(indexPath)
	if err != nil || !bytes.Equal(beforeIndex, afterIndex) {
		t.Fatalf("scan changed the Git index: %v", err)
	}
	wantCalls := 3
	for _, file := range scan.Files {
		if file.IndexMode == "160000" {
			wantCalls = 5
		}
	}
	if scan.Metrics.GitCommands != wantCalls {
		t.Fatalf("scan used %d Git commands, want %d", scan.Metrics.GitCommands, wantCalls)
	}
	for _, check := range []struct {
		got  []string
		args []string
	}{
		{scan.Basis.Staged, []string{"diff", "--cached", "--name-only", "--no-renames", "-z", "--"}},
		{scan.Basis.Unstaged, []string{"diff", "--name-only", "--no-renames", "-z", "--"}},
		{scan.Basis.Untracked, []string{"ls-files", "--others", "--exclude-standard", "-z"}},
	} {
		output := runFixtureGitOutput(t, root, check.args...)
		want := []string{}
		if output != "" {
			want = strings.Split(strings.TrimSuffix(output, "\x00"), "\x00")
			sort.Strings(want)
		}
		if !reflect.DeepEqual(check.got, want) {
			t.Fatalf("%v: got %q, want %q", check.args, check.got, want)
		}
	}
	for _, check := range []struct {
		got  *string
		args []string
	}{
		{scan.Basis.Branch, []string{"symbolic-ref", "--quiet", "--short", "HEAD"}},
		{scan.Basis.Head, []string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"}},
	} {
		output, present, err := runOptionalGit(context.Background(), root, check.args...)
		if err != nil || present != (check.got != nil) || (present && *check.got != strings.TrimSpace(string(output))) {
			t.Fatalf("%v: got %v, want %q, present=%v error=%v", check.args, check.got, output, present, err)
		}
	}
	index := runFixtureGitOutput(t, root, "ls-files", "--stage", "-z")
	wantHash := sha256.Sum256([]byte(index))
	if scan.Basis.IndexDigest != "sha256:"+hex.EncodeToString(wantHash[:]) {
		t.Fatalf("index digest differs from canonical Git output: %#v", scan.Basis)
	}
	incremental, err := repo.IncrementalScan(context.Background(), scan.Files)
	if err != nil || !scan.Basis.SameState(incremental.Basis) || scan.RepositoryDigest != incremental.RepositoryDigest ||
		!reflect.DeepEqual(scan.Files, incremental.Files) {
		t.Fatalf("incremental scan differs from full scan: %v", err)
	}
}

func createScanFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	runFixtureGit(t, root, "config", "user.name", "RepoCue Test")
	runFixtureGit(t, root, "config", "user.email", "repocue@example.invalid")
	writeScanFixture(t, root, "README.md", "# Example\n")
	writeScanFixture(t, root, ".gitignore", "ignored.tmp\n")
	runFixtureGit(t, root, "add", ".")
	runFixtureGit(t, root, "commit", "-m", "Initialize fixture")
	return root
}

func writeScanFixture(t *testing.T, root, path, content string) {
	t.Helper()
	fullPath := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
