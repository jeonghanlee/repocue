package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestReadStableFileStreamsDigestAndBoundedPrefix(t *testing.T) {
	content := bytes.Repeat([]byte("0123456789abcdef"), 128*1024)
	path := filepath.Join(t.TempDir(), "large.bin")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	digest, prefix, size, after, err := readStableFile(context.Background(), path, info)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest := sha256.Sum256(content)
	if digest != "sha256:"+hex.EncodeToString(wantDigest[:]) || size != int64(len(content)) {
		t.Fatalf("unexpected streamed file result: digest=%q size=%d", digest, size)
	}
	if len(prefix) != 8192 || !bytes.Equal(prefix, content[:8192]) || !sameFileInfo(info, after) {
		t.Fatal("streamed file prefix or stable observation is incorrect")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, _, err := readStableFile(canceled, path, info); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled file read returned %v", err)
	}
}

func TestFullScanStreamsLargeTrackedFile(t *testing.T) {
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	runFixtureGit(t, root, "config", "user.name", "RepoCue Test")
	runFixtureGit(t, root, "config", "user.email", "repocue@example.invalid")
	content := bytes.Repeat([]byte("0123456789abcdef"), 128*1024)
	content[0] = 0
	path := filepath.Join(root, "large.bin")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", "large.bin")
	runFixtureGit(t, root, "commit", "-m", "Add large file")
	repository, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := repository.FullScan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantDigest := sha256.Sum256(content)
	if len(scan.Files) != 1 || scan.Files[0].ContentDigest != "sha256:"+hex.EncodeToString(wantDigest[:]) ||
		scan.Files[0].SizeBytes != int64(len(content)) || scan.Files[0].FileType != "binary" ||
		scan.Metrics.BytesScanned != int64(len(content)) {
		t.Fatalf("large-file scan result is incorrect: %#v", scan)
	}
}

func TestFullScanSupportsRepositoryWithoutCommit(t *testing.T) {
	root := t.TempDir()
	command := exec.Command("git", "-C", root, "init", "-b", "main")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("initialize Git repository: %v: %s", err, output)
	}
	repository, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	scan, err := repository.FullScan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if scan.Basis.Head != nil {
		t.Fatalf("got HEAD %q for repository without a commit", *scan.Basis.Head)
	}
	if scan.Basis.Branch == nil || *scan.Basis.Branch != "main" {
		t.Fatalf("unexpected branch: %#v", scan.Basis.Branch)
	}
	if len(scan.Files) != 0 {
		t.Fatalf("got %d tracked files, want 0", len(scan.Files))
	}
}

func TestContextFactsDeterministicEntryPointsAndRecentCommits(t *testing.T) {
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	runFixtureGit(t, root, "config", "user.name", "RepoCue Test")
	runFixtureGit(t, root, "config", "user.email", "repocue@example.invalid")
	files := map[string]string{
		"Makefile":          "build:\n\t@true\n.PHONY: build\ntest: build\n\t@true\n",
		"cmd/zeta/main.go":  "package main\n",
		"cmd/alpha/main.go": "package main\n",
		"src/__main__.py":   "def main():\n    pass\n",
		"docs/odd name.md":  "# Odd\n",
	}
	for path, content := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runFixtureGit(t, root, "add", ".")
	runFixtureGit(t, root, "commit", "-m", "Add entry points")
	if err := os.WriteFile(filepath.Join(root, "docs", "odd name.md"), []byte("# Updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", "docs/odd name.md")
	runFixtureGit(t, root, "commit", "-m", "Update documentation")

	repository, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := repository.ContextFacts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := repository.ContextFacts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(first.RecentCommits, second.RecentCommits) ||
		!slices.Equal(first.EntryPoints, second.EntryPoints) || !slices.Equal(first.MakeTargets, second.MakeTargets) {
		t.Fatalf("context facts are not deterministic: %#v %#v", first, second)
	}
	wantEntryPoints := []string{"Makefile", "cmd/alpha/main.go", "cmd/zeta/main.go", "src/__main__.py"}
	if !slices.Equal(first.EntryPoints, wantEntryPoints) {
		t.Fatalf("entry points = %#v, want %#v", first.EntryPoints, wantEntryPoints)
	}
	if !slices.Equal(first.MakeTargets, []string{"build", "test"}) {
		t.Fatalf("make targets = %#v", first.MakeTargets)
	}
	if !slices.Equal(first.RecentCommits, []string{"Update documentation", "Add entry points"}) {
		t.Fatalf("recent commits = %#v", first.RecentCommits)
	}

	head := runFixtureGitOutput(t, root, "rev-parse", "HEAD")
	runFixtureGit(t, root, "checkout", "--detach", strings.TrimSpace(head))
	detached, err := repository.ContextFacts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(detached.RecentCommits, first.RecentCommits) {
		t.Fatalf("detached HEAD changed context facts: %#v", detached)
	}
}

func TestContextFactsSupportsEmptyHistory(t *testing.T) {
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	repository, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	facts, err := repository.ContextFacts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(facts.RecentCommits) != 0 || len(facts.EntryPoints) != 0 || len(facts.MakeTargets) != 0 {
		t.Fatalf("unexpected empty-history facts: %#v", facts)
	}
}

func TestStateEvidenceChangesWithTrackedContent(t *testing.T) {
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	runFixtureGit(t, root, "config", "user.name", "RepoCue Test")
	runFixtureGit(t, root, "config", "user.email", "repocue@example.invalid")
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte("# Initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", "README.md")
	runFixtureGit(t, root, "commit", "-m", "Initialize fixture")
	repository, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	clean, err := repository.StateEvidence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# Modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err := repository.StateEvidence(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if clean.PorcelainStatusDigest == dirty.PorcelainStatusDigest || clean.BinaryDiffDigest == dirty.BinaryDiffDigest {
		t.Fatalf("state evidence did not change: clean=%#v dirty=%#v", clean, dirty)
	}
}

func TestFullScanRejectsAssumeUnchanged(t *testing.T) {
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	runFixtureGit(t, root, "config", "user.name", "RepoCue Test")
	runFixtureGit(t, root, "config", "user.email", "repocue@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runFixtureGit(t, root, "add", "README.md")
	runFixtureGit(t, root, "commit", "-m", "Initialize fixture")
	runFixtureGit(t, root, "update-index", "--assume-unchanged", "README.md")
	repository, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.FullScan(context.Background()); err == nil {
		t.Fatal("scan unexpectedly accepted an assume-unchanged path")
	}
}

func TestFullScanRejectsFileChangedAfterItWasRead(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the test observes the real scanner file descriptor through procfs")
	}
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	runFixtureGit(t, root, "config", "user.name", "RepoCue Test")
	runFixtureGit(t, root, "config", "user.email", "repocue@example.invalid")
	linkPath := filepath.Join(root, "a-link")
	largePath := filepath.Join(root, "z.bin")
	if err := os.Symlink("initial", linkPath); err != nil {
		t.Fatal(err)
	}
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
	runFixtureGit(t, root, "add", "a-link", "z.bin")
	runFixtureGit(t, root, "commit", "-m", "Initialize fixture")
	if err := os.Remove(linkPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("first", linkPath); err != nil {
		t.Fatal(err)
	}
	repository, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	mutation := make(chan error, 1)
	go mutateWhenOpen(largePath, linkPath, mutation)
	_, scanErr := repository.FullScan(context.Background())
	if err := <-mutation; err != nil {
		t.Fatal(err)
	}
	if scanErr == nil {
		t.Fatal("scan accepted a repository file changed after its content was read")
	}
}

func mutateWhenOpen(watchedPath, linkPath string, result chan<- error) {
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
			if target != watchedPath {
				continue
			}
			if err := os.Remove(linkPath); err != nil {
				result <- err
				return
			}
			result <- os.Symlink("second", linkPath)
			return
		}
		runtime.Gosched()
	}
	result <- errors.New("scanner did not open the watched file before the deadline")
}

func runFixtureGit(t *testing.T, root string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	command := exec.Command("git", commandArgs...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v: %s", args, err, output)
	}
}

func runFixtureGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	command := exec.Command("git", commandArgs...)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return string(output)
}
