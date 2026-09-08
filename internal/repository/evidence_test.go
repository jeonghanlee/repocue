package repository

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestContentEvidenceDetectsUntrackedContentWithoutChangingCoreState(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	path := filepath.Join(root, "untracked.txt")
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.CaptureEvidence(ctx)
	if err != nil {
		t.Fatal(err)
	}
	legacyBefore, err := repo.StateEvidence(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("other"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := repo.CaptureEvidence(ctx)
	if err != nil {
		t.Fatal(err)
	}
	legacyAfter, err := repo.StateEvidence(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if legacyBefore != legacyAfter || before.Scan.RepositoryDigest != after.Scan.RepositoryDigest ||
		before.Scan.Basis.StatusDigest != after.Scan.Basis.StatusDigest {
		t.Fatal("content-only untracked edit unexpectedly changed the legacy/core basis")
	}
	if before.Evidence.Fingerprint == after.Evidence.Fingerprint || !after.Evidence.Complete {
		t.Fatal("strong evidence missed a same-path, same-size content change")
	}
	if after.Evidence.BytesRead != 5 || len(after.Scan.Files) != 0 {
		t.Fatalf("untracked content leaked into core scope: %+v", after)
	}
	repeat, err := repo.CaptureEvidence(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if repeat.Evidence.Fingerprint != after.Evidence.Fingerprint || !reflect.DeepEqual(repeat.Untracked, after.Untracked) {
		t.Fatal("content evidence is not deterministic")
	}
}

func TestContentEvidenceIgnoredFilesSymlinksAndCancellation(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "external")
	if err := os.WriteFile(outside, []byte(strings.Repeat("outside", 1024)), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	repo, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := repo.CaptureEvidence(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Untracked) != 2 || before.Evidence.BytesRead != int64(len("ignored\n")+len(outside)) {
		t.Fatalf("wrong evidence scope: %+v", before)
	}
	if err := os.WriteFile(outside, []byte("changed outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored"), []byte("changed ignored"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := repo.CaptureEvidence(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if before.Evidence.Fingerprint != after.Evidence.Fingerprint {
		t.Fatal("evidence followed a symlink or included ignored content")
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside+"-missing", link); err != nil {
		t.Fatal(err)
	}
	changed, err := repo.CaptureEvidence(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.Evidence.Fingerprint == changed.Evidence.Fingerprint {
		t.Fatal("evidence missed changed symlink target text")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if result, err := repo.CaptureEvidence(cancelled); err == nil || result.Evidence.Complete {
		t.Fatal("cancelled capture returned complete evidence")
	}
}

func TestContentEvidenceRejectsUnreadableUntrackedFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires ordinary filesystem permission enforcement")
	}
	root := t.TempDir()
	runFixtureGit(t, root, "init", "-b", "main")
	path := filepath.Join(root, "unreadable")
	if err := os.WriteFile(path, []byte("not readable"), 0o600); err != nil {
		t.Fatal(err)
	}
	repo, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o600) })
	if captured, err := repo.CaptureEvidence(context.Background()); err == nil || captured.Evidence.Complete {
		t.Fatal("unreadable evidence was silently omitted")
	}
}
