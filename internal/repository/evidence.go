package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jeonghanlee/repocue/internal/model"
)

const EvidenceAlgorithm = "repocue/content-evidence-1"

// ContentEvidence contains hashes and coverage, never paths or file contents.
type ContentEvidence struct {
	Algorithm       string    `json:"algorithm"`
	Fingerprint     string    `json:"fingerprint"`
	RepositoryID    string    `json:"repository_id"`
	StatusDigest    string    `json:"status_digest"`
	TrackedDigest   string    `json:"tracked_digest"`
	UntrackedDigest string    `json:"untracked_digest"`
	Complete        bool      `json:"complete"`
	ObservedAt      time.Time `json:"observed_at"`
	DurationMS      float64   `json:"duration_ms"`
	FilesRead       int       `json:"files_read"`
	BytesRead       int64     `json:"bytes_read"`
	GitCommands     int       `json:"git_commands"`
}

type EvidenceEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Mode   uint32 `json:"mode"`
	Digest string `json:"digest"`
}

type EvidenceCapture struct {
	Evidence  ContentEvidence
	Scan      model.Scan
	Untracked []EvidenceEntry
}

// CaptureEvidence observes an idle checkout without writing the repository or
// cache. Gitlinks retain the core scanner's non-recursive submodule boundary.
func (r *Repository) CaptureEvidence(ctx context.Context) (EvidenceCapture, error) {
	started := time.Now().UTC()
	scan, err := r.FullScan(ctx)
	if err != nil {
		return EvidenceCapture{}, err
	}
	entries := make([]EvidenceEntry, 0, len(scan.Basis.Untracked))
	readBytes := scan.Metrics.BytesScanned
	paths := append([]string(nil), scan.Basis.Untracked...)
	sort.Strings(paths)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return EvidenceCapture{}, err
		}
		entry, size, err := r.evidenceEntry(ctx, path)
		if err != nil {
			return EvidenceCapture{}, fmt.Errorf("capture untracked evidence: %w", err)
		}
		entries = append(entries, entry)
		readBytes += size
	}
	untracked, err := json.Marshal(entries)
	if err != nil {
		return EvidenceCapture{}, err
	}
	evidence := ContentEvidence{
		Algorithm: EvidenceAlgorithm, RepositoryID: r.ID,
		StatusDigest: scan.Basis.StatusDigest, TrackedDigest: scan.RepositoryDigest,
		UntrackedDigest: evidenceDigest(untracked), Complete: true,
		ObservedAt: started, FilesRead: scan.Metrics.FilesScanned + len(entries),
		BytesRead: readBytes, GitCommands: scan.Metrics.GitCommands,
	}
	identity, err := json.Marshal([]string{evidence.Algorithm, evidence.RepositoryID,
		evidence.StatusDigest, evidence.TrackedDigest, evidence.UntrackedDigest})
	if err != nil {
		return EvidenceCapture{}, err
	}
	evidence.Fingerprint = evidenceDigest(identity)
	evidence.DurationMS = float64(time.Since(started)) / float64(time.Millisecond)
	return EvidenceCapture{Evidence: evidence, Scan: scan, Untracked: entries}, nil
}

func (r *Repository) evidenceEntry(ctx context.Context, path string) (EvidenceEntry, int64, error) {
	if filepath.IsAbs(path) || !filepath.IsLocal(path) {
		return EvidenceEntry{}, 0, fmt.Errorf("non-local evidence path")
	}
	full := filepath.Join(r.Root, filepath.FromSlash(path))
	parent, err := filepath.EvalSymlinks(filepath.Dir(full))
	if err != nil {
		return EvidenceEntry{}, 0, err
	}
	relative, err := filepath.Rel(r.Root, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return EvidenceEntry{}, 0, fmt.Errorf("evidence parent escapes repository")
	}
	info, err := os.Lstat(full)
	if err != nil {
		return EvidenceEntry{}, 0, err
	}
	entry := EvidenceEntry{Path: path, Mode: uint32(info.Mode().Perm())}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		if err != nil {
			return EvidenceEntry{}, 0, err
		}
		entry.Kind, entry.Digest = "symlink", evidenceDigest([]byte(target))
		return entry, int64(len(target)), nil
	}
	if !info.Mode().IsRegular() {
		return EvidenceEntry{}, 0, fmt.Errorf("unsupported untracked entry kind")
	}
	file, err := os.Open(full)
	if err != nil {
		return EvidenceEntry{}, 0, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return EvidenceEntry{}, 0, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return EvidenceEntry{}, 0, fmt.Errorf("untracked entry changed during capture")
	}
	hash := sha256.New()
	count, err := io.Copy(hash, evidenceReader{ctx: ctx, reader: file})
	if err != nil {
		return EvidenceEntry{}, 0, err
	}
	entry.Kind, entry.Digest = "regular", "sha256:"+hex.EncodeToString(hash.Sum(nil))
	return entry, count, nil
}

type evidenceReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r evidenceReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

func evidenceDigest(value []byte) string {
	hash := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(hash[:])
}
