package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	locationVersion   = 1
	locationFile      = "location.json"
	locationReadLimit = 4096
	locationConfigMax = 1 << 20
)

// A location hint is disposable; it never contains snapshot or freshness state.
type locationHint struct {
	Version int    `json:"version"`
	Root    string `json:"root"`
	GitDir  string `json:"git_dir"`
	Guard   string `json:"guard"`
}

// OpenCached reuses only a previously Git-verified ordinary repository root.
// Cache read failures are misses. A write failure is a nonfatal warning;
// repository discovery and cancellation errors remain fatal.
func OpenCached(ctx context.Context, path, cacheRoot string) (repo *Repository, warning error, err error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	candidate, hint, eligible := simpleLocation(path)
	if !eligible {
		repo, err = Open(ctx, path)
		return repo, nil, err
	}
	locationPath := filepath.Join(cacheRoot, candidate.ID, locationFile)
	if stored, ok := readLocation(locationPath); ok && stored == hint {
		return candidate, nil, nil
	}
	repo, err = Open(ctx, path)
	if err != nil {
		return nil, nil, err
	}
	if *repo != *candidate {
		return repo, nil, nil
	}
	_, after, eligible := simpleLocation(path)
	if !eligible || after != hint {
		return repo, nil, nil
	}
	if err := writeLocation(locationPath, hint); err != nil {
		return repo, fmt.Errorf("repository location cache was not saved: %w", err), nil
	}
	return repo, nil, nil
}

// Repository discovery reads repository-local format/worktree configuration.
// Nonstandard layouts, includes, worktree configuration, and Git environment
// overrides stay on the Git path instead of duplicating its discovery rules.
func simpleLocation(path string) (*Repository, locationHint, bool) {
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "GIT_") && !strings.HasPrefix(key, "GIT_TRACE") && key != "GIT_PAGER" {
			return nil, locationHint{}, false
		}
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return nil, locationHint{}, false
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil || strings.ContainsAny(root, "\r\n") {
		return nil, locationHint{}, false
	}
	gitDir := filepath.Join(root, ".git")
	for _, name := range []string{"commondir", "gitdir", "config.worktree"} {
		if _, err := os.Lstat(filepath.Join(gitDir, name)); !os.IsNotExist(err) {
			return nil, locationHint{}, false
		}
	}
	command, err := exec.LookPath("git")
	if err != nil {
		return nil, locationHint{}, false
	}
	command, err = filepath.EvalSymlinks(command)
	if err != nil {
		return nil, locationHint{}, false
	}
	hasher := sha256.New()
	for _, name := range []string{root, gitDir, filepath.Join(gitDir, "objects"), filepath.Join(gitDir, "refs"), command} {
		info, err := os.Lstat(name)
		if err != nil || (name != command && !info.IsDir()) || (name == command && !info.Mode().IsRegular()) {
			return nil, locationHint{}, false
		}
		// Root contents do not determine repository location. Metadata of the
		// Git directories and executable invalidates replaced topology/tools.
		fmt.Fprintf(hasher, "%s\x00%d\x00", name, info.Mode())
		if name != root {
			fmt.Fprintf(hasher, "%d\x00%d\x00", info.Size(), info.ModTime().UnixNano())
		}
	}
	for _, name := range []string{"config", "HEAD"} {
		path := filepath.Join(gitDir, name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, locationHint{}, false
		}
		data, err := readLimited(path, locationConfigMax)
		if err != nil {
			return nil, locationHint{}, false
		}
		if name == "config" {
			lower := bytes.ToLower(data)
			if bytes.Contains(lower, []byte("include")) || bytes.Contains(lower, []byte("worktree")) ||
				bytes.Contains(lower, []byte("extensions")) {
				return nil, locationHint{}, false
			}
		}
		fmt.Fprintf(hasher, "%s\x00%d\x00%d\x00", name, info.Mode(), info.ModTime().UnixNano())
		hasher.Write(data)
		hasher.Write([]byte{0})
	}
	repo := newRepository(root, gitDir)
	return repo, locationHint{Version: locationVersion, Root: root, GitDir: gitDir,
		Guard: "sha256:" + hex.EncodeToString(hasher.Sum(nil))}, true
}

func readLocation(path string) (locationHint, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return locationHint{}, false
	}
	data, err := readLimited(path, locationReadLimit)
	if err != nil {
		return locationHint{}, false
	}
	var hint locationHint
	if err := json.Unmarshal(data, &hint); err != nil {
		return locationHint{}, false
	}
	return hint, true
}

func readLimited(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err == nil && int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds the location cache read limit", path)
	}
	return data, err
}

func writeLocation(path string, hint locationHint) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".location-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := json.NewEncoder(file).Encode(hint); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
