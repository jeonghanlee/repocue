package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jeonghanlee/repocue/internal/model"
)

type Repository struct {
	Root   string
	GitDir string
	ID     string
	Name   string
}

type ContextFacts struct {
	RecentCommits []string `json:"recent_commits"`
	EntryPoints   []string `json:"entry_points"`
	MakeTargets   []string `json:"make_targets"`
}

type StateEvidence struct {
	PorcelainStatusDigest string `json:"porcelain_status_digest"`
	BinaryDiffDigest      string `json:"binary_diff_digest"`
}

var makeTargetPattern = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_.-]*):(?:[^=]|$)`)

type indexEntry struct {
	Path string
	Mode string
	OID  string
}

type scanSession struct {
	repository  *Repository
	gitCommands int
}

func Open(ctx context.Context, path string) (*Repository, error) {
	output, err := runGit(ctx, path, "rev-parse", "--show-toplevel", "--absolute-git-dir")
	if err != nil {
		return nil, fmt.Errorf("discover Git repository: %w", err)
	}
	paths := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	// rev-parse has no NUL-delimited path output. Query each path separately
	// when embedded newlines make the combined output ambiguous.
	if len(paths) != 2 {
		rootOutput, err := runGit(ctx, path, "rev-parse", "--show-toplevel")
		if err != nil {
			return nil, fmt.Errorf("discover Git repository: %w", err)
		}
		gitDirOutput, err := runGit(ctx, path, "rev-parse", "--absolute-git-dir")
		if err != nil {
			return nil, fmt.Errorf("resolve Git directory: %w", err)
		}
		paths = []string{strings.TrimSuffix(string(rootOutput), "\n"), strings.TrimSuffix(string(gitDirOutput), "\n")}
	}
	root, err := filepath.Abs(paths[0])
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root symlinks: %w", err)
	}
	gitDir := paths[1]
	return newRepository(root, gitDir), nil
}

func newRepository(root, gitDir string) *Repository {
	digest := sha256.Sum256([]byte(root))
	return &Repository{
		Root:   root,
		GitDir: gitDir,
		ID:     "repo-" + hex.EncodeToString(digest[:12]),
		Name:   filepath.Base(root),
	}
}

// FullScan captures one Git basis and assumes the repository remains unchanged
// until the scan returns. Concurrent filesystem mutation is not validated.
func (r *Repository) FullScan(ctx context.Context) (model.Scan, error) {
	started := time.Now()
	session := &scanSession{repository: r}
	basis, entries, err := session.captureBasis(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	files := make([]model.File, 0, len(entries))
	staged := pathSet(basis.Staged)
	unstaged := pathSet(basis.Unstaged)
	var bytesScanned int64
	for _, entry := range entries {
		file, readBytes, err := r.scanEntry(ctx, entry, staged, unstaged)
		if err != nil {
			return model.Scan{}, err
		}
		files = append(files, file)
		bytesScanned += readBytes
	}
	if err := ctx.Err(); err != nil {
		return model.Scan{}, err
	}
	repositoryHash, err := repositoryDigest(files)
	if err != nil {
		return model.Scan{}, err
	}
	return model.Scan{
		Basis:            basis,
		Files:            files,
		RepositoryDigest: repositoryHash,
		Metrics: model.ScanMetrics{
			StartedAt:    started,
			Duration:     time.Since(started),
			GitCommands:  session.gitCommands,
			FilesScanned: len(entries),
			BytesScanned: bytesScanned,
		},
	}, nil
}

// IncrementalScan captures one Git basis and reads changed tracked files.
// The repository must remain unchanged until the scan returns.
func (r *Repository) IncrementalScan(ctx context.Context, previous []model.File) (model.Scan, error) {
	started := time.Now()
	session := &scanSession{repository: r}
	basis, entries, err := session.captureBasis(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	previousByPath := make(map[string]model.File, len(previous))
	for _, file := range previous {
		previousByPath[file.Path] = file
	}
	currentPaths := make(map[string]struct{}, len(entries))
	files := make([]model.File, 0, len(entries))
	staged := pathSet(basis.Staged)
	unstaged := pathSet(basis.Unstaged)
	filesScanned := 0
	var bytesScanned int64
	for _, entry := range entries {
		currentPaths[entry.Path] = struct{}{}
		prior, found := previousByPath[entry.Path]
		exists := fileExists(filepath.Join(r.Root, entry.Path))
		currentState := workingState(entry.Path, staged, unstaged, exists)
		mustScan := !found || prior.IndexMode != entry.Mode || prior.IndexObject != entry.OID ||
			prior.WorkingState != "clean" || currentState != "clean" ||
			prior.Exists != exists
		if !mustScan {
			files = append(files, prior)
			continue
		}
		file, readBytes, err := r.scanEntry(ctx, entry, staged, unstaged)
		if err != nil {
			return model.Scan{}, err
		}
		files = append(files, file)
		filesScanned++
		bytesScanned += readBytes
	}
	for path := range previousByPath {
		if _, found := currentPaths[path]; !found {
			filesScanned++
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if err := ctx.Err(); err != nil {
		return model.Scan{}, err
	}
	repositoryHash, err := repositoryDigest(files)
	if err != nil {
		return model.Scan{}, err
	}
	return model.Scan{
		Basis:            basis,
		Files:            files,
		RepositoryDigest: repositoryHash,
		Metrics: model.ScanMetrics{
			StartedAt:    started,
			Duration:     time.Since(started),
			GitCommands:  session.gitCommands,
			FilesScanned: filesScanned,
			BytesScanned: bytesScanned,
		},
	}, nil
}

func (r *Repository) ContextFacts(ctx context.Context) (ContextFacts, error) {
	session := &scanSession{repository: r}
	entries, _, err := session.indexEntries(ctx)
	if err != nil {
		return ContextFacts{}, err
	}
	_, hasHead, err := session.runOptionalGit(ctx, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return ContextFacts{}, fmt.Errorf("inspect recent commit basis: %w", err)
	}
	facts := ContextFacts{RecentCommits: []string{}, EntryPoints: []string{}, MakeTargets: []string{}}
	if hasHead {
		recentOutput, err := session.runGit(ctx, "log", "-5", "--format=%s")
		if err != nil {
			return ContextFacts{}, fmt.Errorf("read recent commits: %w", err)
		}
		for _, subject := range strings.Split(strings.TrimSpace(string(recentOutput)), "\n") {
			if subject != "" {
				facts.RecentCommits = append(facts.RecentCommits, subject)
			}
		}
	}
	for _, entry := range entries {
		path := filepath.ToSlash(entry.Path)
		if isEntryPointPath(path) {
			facts.EntryPoints = append(facts.EntryPoints, path)
		}
		if filepath.Base(path) != "Makefile" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(path)))
		if err != nil {
			return ContextFacts{}, fmt.Errorf("read %s targets: %w", path, err)
		}
		for _, line := range strings.Split(string(content), "\n") {
			match := makeTargetPattern.FindStringSubmatch(line)
			if len(match) == 2 && !strings.HasPrefix(match[1], ".") {
				facts.MakeTargets = append(facts.MakeTargets, match[1])
			}
		}
	}
	facts.EntryPoints = unique(facts.EntryPoints)
	facts.MakeTargets = unique(facts.MakeTargets)
	return facts, nil
}

// ContextFactsAt returns live-derived context facts only when the repository
// remains identical to the supplied persisted snapshot for the whole read.
func (r *Repository) ContextFactsAt(ctx context.Context, expected model.Snapshot, files []model.File) (ContextFacts, error) {
	if _, err := r.ValidateSnapshot(ctx, expected, files); err != nil {
		return ContextFacts{}, err
	}
	facts, err := r.ContextFacts(ctx)
	if err != nil {
		return ContextFacts{}, err
	}
	if _, err := r.ValidateSnapshot(ctx, expected, files); err != nil {
		return ContextFacts{}, err
	}
	return facts, nil
}

// ValidateSnapshot verifies that the live Git and tracked-file state still
// matches a persisted snapshot and returns the observed scan on success.
func (r *Repository) ValidateSnapshot(ctx context.Context, expected model.Snapshot, files []model.File) (model.Scan, error) {
	scan, err := r.IncrementalScan(ctx, files)
	if err != nil {
		return model.Scan{}, err
	}
	if scan.Basis.StatusDigest != expected.Basis.StatusDigest || scan.RepositoryDigest != expected.RepositoryDigest {
		return model.Scan{}, errors.New("repository no longer matches the persisted snapshot")
	}
	return scan, nil
}

func (r *Repository) StateEvidence(ctx context.Context) (StateEvidence, error) {
	session := &scanSession{repository: r}
	status, err := session.runGit(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return StateEvidence{}, fmt.Errorf("capture porcelain status: %w", err)
	}
	staged, err := session.runGit(ctx, "diff", "--binary", "--no-ext-diff", "--cached", "--")
	if err != nil {
		return StateEvidence{}, fmt.Errorf("capture staged binary diff: %w", err)
	}
	unstaged, err := session.runGit(ctx, "diff", "--binary", "--no-ext-diff", "--")
	if err != nil {
		return StateEvidence{}, fmt.Errorf("capture unstaged binary diff: %w", err)
	}
	statusHash := sha256.Sum256(status)
	diffContent := make([]byte, 0, len(staged)+len(unstaged)+1)
	diffContent = append(diffContent, staged...)
	diffContent = append(diffContent, 0)
	diffContent = append(diffContent, unstaged...)
	diffHash := sha256.Sum256(diffContent)
	return StateEvidence{
		PorcelainStatusDigest: "sha256:" + hex.EncodeToString(statusHash[:]),
		BinaryDiffDigest:      "sha256:" + hex.EncodeToString(diffHash[:]),
	}, nil
}

func isEntryPointPath(path string) bool {
	base := filepath.Base(path)
	if base == "main.go" || base == "main.py" || base == "__main__.py" {
		return true
	}
	if strings.HasPrefix(path, "cmd/") || strings.HasPrefix(path, "bin/") {
		return true
	}
	return path == "Makefile" || path == "go.mod" || path == "pyproject.toml"
}

func (s *scanSession) captureBasis(ctx context.Context) (model.Basis, []indexEntry, error) {
	output, err := s.runGit(ctx, "--no-optional-locks", "status", "--porcelain=v2", "--branch",
		"--no-ahead-behind", "--no-renames", "--untracked-files=all", "-z")
	if err != nil {
		return model.Basis{}, nil, fmt.Errorf("read Git status: %w", err)
	}
	basis, err := parseStatus(output)
	if err != nil {
		return model.Basis{}, nil, err
	}
	// Git's short symbolic ref preserves namespace disambiguation and distinguishes
	// a branch literally named "(detached)" from an actual detached HEAD.
	branchOutput, attached, err := s.runOptionalGit(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return model.Basis{}, nil, fmt.Errorf("read branch: %w", err)
	}
	if attached {
		branch := strings.TrimSpace(string(branchOutput))
		basis.Branch = &branch
	}
	entries, indexDigest, err := s.indexEntries(ctx)
	if err != nil {
		return model.Basis{}, nil, err
	}
	// Status and diff have different defaults for untracked submodule content.
	// Preserve diff's existing configuration-sensitive semantics for gitlinks.
	for _, entry := range entries {
		if entry.Mode != "160000" {
			continue
		}
		basis.Staged, err = s.diffPaths(ctx, true)
		if err != nil {
			return model.Basis{}, nil, err
		}
		basis.Unstaged, err = s.diffPaths(ctx, false)
		if err != nil {
			return model.Basis{}, nil, err
		}
		break
	}
	branch, head := basis.Branch, basis.Head
	staged, unstaged, untracked := basis.Staged, basis.Unstaged, basis.Untracked
	state := struct {
		Branch    *string  `json:"branch"`
		Head      *string  `json:"head"`
		Staged    []string `json:"staged"`
		Unstaged  []string `json:"unstaged"`
		Untracked []string `json:"untracked"`
		Index     string   `json:"index_digest"`
	}{branch, head, staged, unstaged, untracked, indexDigest}
	worktree := struct {
		Staged    []string `json:"staged"`
		Unstaged  []string `json:"unstaged"`
		Untracked []string `json:"untracked"`
	}{staged, unstaged, untracked}
	statusDigest, err := digestJSON(state)
	if err != nil {
		return model.Basis{}, nil, err
	}
	workingTreeDigest, err := digestJSON(worktree)
	if err != nil {
		return model.Basis{}, nil, err
	}
	return model.Basis{
		Branch:            branch,
		Head:              head,
		Dirty:             len(staged)+len(unstaged)+len(untracked) > 0,
		Staged:            staged,
		Unstaged:          unstaged,
		Untracked:         untracked,
		IndexDigest:       indexDigest,
		StatusDigest:      statusDigest,
		WorkingTreeDigest: workingTreeDigest,
		ObservedAt:        time.Now().UTC(),
	}, entries, nil
}

// parseStatus reads HEAD and paths from NUL-delimited porcelain v2 without renames.
// Branch identity is resolved separately with Git's short symbolic ref.
// Unknown headers are ignored; unexpected entry formats fail closed.
func parseStatus(output []byte) (model.Basis, error) {
	basis := model.Basis{Staged: []string{}, Unstaged: []string{}, Untracked: []string{}}
	headSeen := false
	for _, record := range bytes.Split(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		if !utf8.Valid(record) {
			return model.Basis{}, errors.New("Git returned an unsupported path encoding")
		}
		line := string(record)
		switch {
		case strings.HasPrefix(line, "# branch.oid "):
			headSeen = true
			value := strings.TrimPrefix(line, "# branch.oid ")
			if value != "(initial)" {
				basis.Head = &value
			}
		case strings.HasPrefix(line, "# "):
			continue
		case strings.HasPrefix(line, "1 "):
			fields := strings.SplitN(line, " ", 9)
			if len(fields) != 9 || len(fields[1]) != 2 || fields[8] == "" {
				return model.Basis{}, errors.New("Git returned an invalid status entry")
			}
			if fields[1][0] != '.' {
				basis.Staged = append(basis.Staged, fields[8])
			}
			if fields[1][1] != '.' {
				basis.Unstaged = append(basis.Unstaged, fields[8])
			}
		case strings.HasPrefix(line, "? ") && len(line) > 2:
			basis.Untracked = append(basis.Untracked, line[2:])
		case strings.HasPrefix(line, "u "):
			return model.Basis{}, errors.New("unmerged index entries are not supported")
		default:
			return model.Basis{}, errors.New("Git returned an unsupported status entry")
		}
	}
	if !headSeen {
		return model.Basis{}, errors.New("Git status omitted the HEAD basis")
	}
	sort.Strings(basis.Staged)
	sort.Strings(basis.Unstaged)
	sort.Strings(basis.Untracked)
	basis.Staged = unique(basis.Staged)
	basis.Unstaged = unique(basis.Unstaged)
	basis.Untracked = unique(basis.Untracked)
	return basis, nil
}

func (s *scanSession) indexEntries(ctx context.Context) ([]indexEntry, string, error) {
	output, err := s.runGit(ctx, "ls-files", "--stage", "-v", "-z")
	if err != nil {
		return nil, "", fmt.Errorf("list tracked files: %w", err)
	}
	entries := make([]indexEntry, 0)
	hasher := sha256.New()
	for _, record := range bytes.Split(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		if len(record) < 3 || record[1] != ' ' {
			return nil, "", errors.New("Git index contains an invalid visibility tag")
		}
		tag := record[0]
		if tag == 'S' || tag >= 'a' && tag <= 'z' {
			return nil, "", errors.New("assume-unchanged and skip-worktree index entries are not supported because freshness cannot be guaranteed")
		}
		metadata, pathBytes, found := bytes.Cut(record[2:], []byte{'\t'})
		if !found || !utf8.Valid(pathBytes) {
			return nil, "", errors.New("Git index contains an unsupported path encoding")
		}
		fields := strings.Fields(string(metadata))
		if len(fields) != 3 || fields[2] != "0" {
			return nil, "", errors.New("Git index contains an unsupported staged entry")
		}
		// Preserve the canonical ls-files --stage -z digest without the tag.
		hasher.Write(record[2:])
		hasher.Write([]byte{0})
		entries = append(entries, indexEntry{Path: string(pathBytes), Mode: fields[0], OID: fields[1]})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func (r *Repository) scanEntry(ctx context.Context, entry indexEntry, staged, unstaged map[string]struct{}) (model.File, int64, error) {
	fullPath := filepath.Join(r.Root, filepath.FromSlash(entry.Path))
	info, err := os.Lstat(fullPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return model.File{}, 0, fmt.Errorf("inspect %s: %w", entry.Path, err)
	}
	exists := err == nil
	file := model.File{
		EntityID:     "file:" + entry.Path,
		Path:         entry.Path,
		IndexMode:    entry.Mode,
		IndexObject:  entry.OID,
		Exists:       exists,
		WorkingState: workingState(entry.Path, staged, unstaged, exists),
	}
	if !exists {
		file.FileType = classifyType(entry.Path, nil, isDocument(entry.Path))
		file.Language = classifyLanguage(entry.Path)
		file.Document = isDocument(entry.Path)
		return file, 0, nil
	}
	if entry.Mode == "160000" {
		file.WorkingTreeMode = "160000"
		file.ContentDigest = "git-object:" + entry.OID
		file.FileType = "gitlink"
		return file, 0, nil
	}
	var contentPrefix []byte
	var contentDigest string
	var sizeBytes int64
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(fullPath)
		if err != nil {
			return model.File{}, 0, fmt.Errorf("read symlink %s: %w", entry.Path, err)
		}
		contentPrefix = []byte(target)
		file.WorkingTreeMode = "120000"
		digest := sha256.Sum256(contentPrefix)
		contentDigest = "sha256:" + hex.EncodeToString(digest[:])
		sizeBytes = int64(len(contentPrefix))
	} else {
		if !info.Mode().IsRegular() {
			return model.File{}, 0, fmt.Errorf("tracked path %s is not a regular file", entry.Path)
		}
		contentDigest, contentPrefix, sizeBytes, _, err = readStableFile(ctx, fullPath, info)
		if err != nil {
			return model.File{}, 0, fmt.Errorf("read %s: %w", entry.Path, err)
		}
		file.WorkingTreeMode = "100644"
		if info.Mode().Perm()&0o111 != 0 {
			file.WorkingTreeMode = "100755"
		}
	}
	file.SizeBytes = sizeBytes
	file.ContentDigest = contentDigest
	file.FileType = classifyType(entry.Path, contentPrefix, isDocument(entry.Path))
	file.Language = classifyLanguage(entry.Path)
	file.Document = isDocument(entry.Path)
	return file, sizeBytes, nil
}

func readStableFile(ctx context.Context, path string, before os.FileInfo) (string, []byte, int64, os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, 0, nil, err
	}
	defer file.Close()
	hasher := sha256.New()
	prefix := make([]byte, 0, 8192)
	buffer := make([]byte, 32*1024)
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return "", nil, 0, nil, err
		}
		count, readErr := file.Read(buffer)
		if count > 0 {
			chunk := buffer[:count]
			if _, err := hasher.Write(chunk); err != nil {
				return "", nil, 0, nil, err
			}
			remaining := cap(prefix) - len(prefix)
			if remaining > len(chunk) {
				remaining = len(chunk)
			}
			if remaining > 0 {
				prefix = append(prefix, chunk[:remaining]...)
			}
			size += int64(count)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", nil, 0, nil, readErr
		}
	}
	after, err := os.Lstat(path)
	if err != nil {
		return "", nil, 0, nil, err
	}
	if !sameFileInfo(before, after) {
		return "", nil, 0, nil, errors.New("file changed while being read")
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), prefix, size, after, nil
}

func sameFileInfo(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Size() == after.Size() &&
		before.Mode() == after.Mode() && before.ModTime().Equal(after.ModTime())
}

func (s *scanSession) runGit(ctx context.Context, args ...string) ([]byte, error) {
	s.gitCommands++
	return runGit(ctx, s.repository.Root, args...)
}

func (s *scanSession) diffPaths(ctx context.Context, staged bool) ([]string, error) {
	args := []string{"diff", "--name-only", "--no-renames", "-z"}
	if staged {
		args = append(args, "--cached")
	}
	output, err := s.runGit(ctx, append(args, "--")...)
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, raw := range bytes.Split(output, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		if !utf8.Valid(raw) {
			return nil, errors.New("Git returned an unsupported path encoding")
		}
		paths = append(paths, string(raw))
	}
	sort.Strings(paths)
	return unique(paths), nil
}

func (s *scanSession) runOptionalGit(ctx context.Context, args ...string) ([]byte, bool, error) {
	s.gitCommands++
	return runOptionalGit(ctx, s.repository.Root, args...)
}

func runGit(ctx context.Context, directory string, args ...string) ([]byte, error) {
	commandArgs := append([]string{"-C", directory}, args...)
	command := exec.CommandContext(ctx, "git", commandArgs...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	output, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitError.Stderr)))
		}
		return nil, err
	}
	return output, nil
}

func runOptionalGit(ctx context.Context, directory string, args ...string) ([]byte, bool, error) {
	commandArgs := append([]string{"-C", directory}, args...)
	command := exec.CommandContext(ctx, "git", commandArgs...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	output, err := command.Output()
	if err == nil {
		return output, true, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return nil, false, nil
	}
	if errors.As(err, &exitError) {
		return nil, false, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitError.Stderr)))
	}
	return nil, false, err
}

func pathSet(paths []string) map[string]struct{} {
	result := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		result[path] = struct{}{}
	}
	return result
}

func workingState(path string, staged, unstaged map[string]struct{}, exists bool) string {
	_, isStaged := staged[path]
	_, isUnstaged := unstaged[path]
	if !exists && (isStaged || isUnstaged) {
		return "deleted"
	}
	if isStaged && isUnstaged {
		return "staged-and-unstaged"
	}
	if isStaged {
		return "staged"
	}
	if isUnstaged {
		return "unstaged"
	}
	return "clean"
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func repositoryDigest(files []model.File) (string, error) {
	return digestJSON(files)
}

func digestJSON(value any) (string, error) {
	serialized, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("serialize repository state: %w", err)
	}
	digest := sha256.Sum256(serialized)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func unique(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func classifyLanguage(path string) string {
	name := strings.ToLower(filepath.Base(path))
	if name == "makefile" {
		return "Makefile"
	}
	extension := strings.ToLower(filepath.Ext(path))
	return map[string]string{
		".go": "Go", ".py": "Python", ".c": "C", ".h": "C",
		".cc": "C++", ".cpp": "C++", ".cxx": "C++", ".hpp": "C++",
		".js": "JavaScript", ".jsx": "JavaScript", ".ts": "TypeScript", ".tsx": "TypeScript",
		".java": "Java", ".rs": "Rust", ".sh": "Shell", ".bash": "Shell",
		".md": "Markdown", ".rst": "reStructuredText", ".json": "JSON",
		".yaml": "YAML", ".yml": "YAML", ".toml": "TOML", ".xml": "XML",
		".sql": "SQL", ".proto": "Protocol Buffers",
	}[extension]
}

func classifyType(path string, content []byte, document bool) string {
	if document {
		return "documentation"
	}
	if bytes.IndexByte(contentPrefix(content, 8192), 0) >= 0 {
		return "binary"
	}
	extension := strings.ToLower(filepath.Ext(path))
	if extension == ".json" || extension == ".yaml" || extension == ".yml" || extension == ".toml" || extension == ".xml" {
		return "configuration"
	}
	if classifyLanguage(path) != "" {
		return "source"
	}
	return "other"
}

func contentPrefix(content []byte, limit int) []byte {
	if len(content) <= limit {
		return content
	}
	return content[:limit]
}

func isDocument(path string) bool {
	normalized := filepath.ToSlash(path)
	base := strings.ToLower(filepath.Base(normalized))
	extension := strings.ToLower(filepath.Ext(normalized))
	if strings.HasPrefix(base, "readme") || base == "agents.md" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(normalized), "docs/") &&
		(extension == ".md" || extension == ".rst" || extension == ".adoc" || extension == ".txt")
}
