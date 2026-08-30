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

type fileObservation struct {
	path          string
	exists        bool
	info          os.FileInfo
	symlinkTarget string
}

func Open(ctx context.Context, path string) (*Repository, error) {
	rootOutput, err := runGit(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("discover Git repository: %w", err)
	}
	root, err := filepath.Abs(strings.TrimSpace(string(rootOutput)))
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root symlinks: %w", err)
	}
	gitDirOutput, err := runGit(ctx, root, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return nil, fmt.Errorf("resolve Git directory: %w", err)
	}
	gitDir := strings.TrimSpace(string(gitDirOutput))
	digest := sha256.Sum256([]byte(root))
	return &Repository{
		Root:   root,
		GitDir: gitDir,
		ID:     "repo-" + hex.EncodeToString(digest[:12]),
		Name:   filepath.Base(root),
	}, nil
}

func (r *Repository) FullScan(ctx context.Context) (model.Scan, error) {
	started := time.Now()
	session := &scanSession{repository: r}
	before, err := session.captureBasis(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	entries, err := session.indexEntries(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	files := make([]model.File, 0, len(entries))
	staged := pathSet(before.Staged)
	unstaged := pathSet(before.Unstaged)
	var bytesScanned int64
	observations := make([]fileObservation, 0, len(entries))
	for _, entry := range entries {
		file, observation, readBytes, err := r.scanEntry(entry, staged, unstaged)
		if err != nil {
			return model.Scan{}, err
		}
		files = append(files, file)
		observations = append(observations, observation)
		bytesScanned += readBytes
	}
	after, err := session.captureBasis(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	if !before.SameState(after) {
		return model.Scan{}, errors.New("repository changed during scan")
	}
	observedAt, err := validateObservations(observations)
	if err != nil {
		return model.Scan{}, err
	}
	finalBasis, err := session.captureBasis(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	if !after.SameState(finalBasis) {
		return model.Scan{}, errors.New("repository changed during scan validation")
	}
	finalBasis.ObservedAt = observedAt
	repositoryHash, err := repositoryDigest(files)
	if err != nil {
		return model.Scan{}, err
	}
	return model.Scan{
		Basis:            finalBasis,
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

func (r *Repository) IncrementalScan(ctx context.Context, previous []model.File) (model.Scan, error) {
	started := time.Now()
	session := &scanSession{repository: r}
	before, err := session.captureBasis(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	entries, err := session.indexEntries(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	previousByPath := make(map[string]model.File, len(previous))
	for _, file := range previous {
		previousByPath[file.Path] = file
	}
	currentPaths := make(map[string]struct{}, len(entries))
	files := make([]model.File, 0, len(entries))
	staged := pathSet(before.Staged)
	unstaged := pathSet(before.Unstaged)
	filesScanned := 0
	var bytesScanned int64
	observations := make([]fileObservation, 0)
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
		file, observation, readBytes, err := r.scanEntry(entry, staged, unstaged)
		if err != nil {
			return model.Scan{}, err
		}
		files = append(files, file)
		observations = append(observations, observation)
		filesScanned++
		bytesScanned += readBytes
	}
	for path := range previousByPath {
		if _, found := currentPaths[path]; !found {
			filesScanned++
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	after, err := session.captureBasis(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	if !before.SameState(after) {
		return model.Scan{}, errors.New("repository changed during scan")
	}
	observedAt, err := validateObservations(observations)
	if err != nil {
		return model.Scan{}, err
	}
	finalBasis, err := session.captureBasis(ctx)
	if err != nil {
		return model.Scan{}, err
	}
	if !after.SameState(finalBasis) {
		return model.Scan{}, errors.New("repository changed during scan validation")
	}
	finalBasis.ObservedAt = observedAt
	repositoryHash, err := repositoryDigest(files)
	if err != nil {
		return model.Scan{}, err
	}
	return model.Scan{
		Basis:            finalBasis,
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
	entries, err := session.indexEntries(ctx)
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

func (s *scanSession) captureBasis(ctx context.Context) (model.Basis, error) {
	branch, err := s.optionalGitValue(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return model.Basis{}, fmt.Errorf("read branch: %w", err)
	}
	head, err := s.optionalGitValue(ctx, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	if err != nil {
		return model.Basis{}, fmt.Errorf("read HEAD: %w", err)
	}
	staged, err := s.pathList(ctx, "diff", "--cached", "--name-only", "--no-renames", "-z", "--")
	if err != nil {
		return model.Basis{}, fmt.Errorf("read staged paths: %w", err)
	}
	unstaged, err := s.pathList(ctx, "diff", "--name-only", "--no-renames", "-z", "--")
	if err != nil {
		return model.Basis{}, fmt.Errorf("read unstaged paths: %w", err)
	}
	untracked, err := s.pathList(ctx, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return model.Basis{}, fmt.Errorf("read untracked paths: %w", err)
	}
	unmerged, err := s.pathList(ctx, "ls-files", "--unmerged", "-z")
	if err != nil {
		return model.Basis{}, fmt.Errorf("read unmerged paths: %w", err)
	}
	if len(unmerged) > 0 {
		return model.Basis{}, errors.New("unmerged index entries are not supported")
	}
	indexOutput, err := s.runGit(ctx, "ls-files", "--stage", "-z")
	if err != nil {
		return model.Basis{}, fmt.Errorf("read index state: %w", err)
	}
	indexHash := sha256.Sum256(indexOutput)
	indexDigest := "sha256:" + hex.EncodeToString(indexHash[:])
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
		return model.Basis{}, err
	}
	workingTreeDigest, err := digestJSON(worktree)
	if err != nil {
		return model.Basis{}, err
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
	}, nil
}

func (s *scanSession) indexEntries(ctx context.Context) ([]indexEntry, error) {
	if err := s.verifyVisibleIndex(ctx); err != nil {
		return nil, err
	}
	output, err := s.runGit(ctx, "ls-files", "--stage", "-z")
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}
	entries := make([]indexEntry, 0)
	for _, record := range bytes.Split(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		metadata, pathBytes, found := bytes.Cut(record, []byte{'\t'})
		if !found || !utf8.Valid(pathBytes) {
			return nil, errors.New("Git index contains an unsupported path encoding")
		}
		fields := strings.Fields(string(metadata))
		if len(fields) != 3 || fields[2] != "0" {
			return nil, errors.New("Git index contains an unsupported staged entry")
		}
		entries = append(entries, indexEntry{Path: string(pathBytes), Mode: fields[0], OID: fields[1]})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func (s *scanSession) verifyVisibleIndex(ctx context.Context) error {
	output, err := s.runGit(ctx, "ls-files", "-v", "-z")
	if err != nil {
		return fmt.Errorf("inspect index visibility: %w", err)
	}
	for _, record := range bytes.Split(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		tag := record[0]
		if tag == 'S' || tag >= 'a' && tag <= 'z' {
			return errors.New("assume-unchanged and skip-worktree index entries are not supported because freshness cannot be guaranteed")
		}
	}
	return nil
}

func (r *Repository) scanEntry(entry indexEntry, staged, unstaged map[string]struct{}) (model.File, fileObservation, int64, error) {
	fullPath := filepath.Join(r.Root, filepath.FromSlash(entry.Path))
	info, err := os.Lstat(fullPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return model.File{}, fileObservation{}, 0, fmt.Errorf("inspect %s: %w", entry.Path, err)
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
	observation := fileObservation{path: fullPath, exists: exists, info: info}
	if !exists {
		file.FileType = classifyType(entry.Path, nil, isDocument(entry.Path))
		file.Language = classifyLanguage(entry.Path)
		file.Document = isDocument(entry.Path)
		return file, observation, 0, nil
	}
	if entry.Mode == "160000" {
		file.WorkingTreeMode = "160000"
		file.ContentDigest = "git-object:" + entry.OID
		file.FileType = "gitlink"
		return file, observation, 0, nil
	}
	var content []byte
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(fullPath)
		if err != nil {
			return model.File{}, fileObservation{}, 0, fmt.Errorf("read symlink %s: %w", entry.Path, err)
		}
		content = []byte(target)
		observation.symlinkTarget = target
		file.WorkingTreeMode = "120000"
	} else {
		if !info.Mode().IsRegular() {
			return model.File{}, fileObservation{}, 0, fmt.Errorf("tracked path %s is not a regular file", entry.Path)
		}
		content, observation.info, err = readStableFile(fullPath, info)
		if err != nil {
			return model.File{}, fileObservation{}, 0, fmt.Errorf("read %s: %w", entry.Path, err)
		}
		file.WorkingTreeMode = "100644"
		if info.Mode().Perm()&0o111 != 0 {
			file.WorkingTreeMode = "100755"
		}
	}
	digest := sha256.Sum256(content)
	file.SizeBytes = int64(len(content))
	file.ContentDigest = "sha256:" + hex.EncodeToString(digest[:])
	file.FileType = classifyType(entry.Path, content, isDocument(entry.Path))
	file.Language = classifyLanguage(entry.Path)
	file.Document = isDocument(entry.Path)
	return file, observation, int64(len(content)), nil
}

func readStableFile(path string, before os.FileInfo) ([]byte, os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, err
	}
	after, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, nil, errors.New("file changed while being read")
	}
	return content, after, nil
}

func validateObservations(observations []fileObservation) (time.Time, error) {
	observedAt := time.Now().UTC()
	for _, observation := range observations {
		info, err := os.Lstat(observation.path)
		if errors.Is(err, os.ErrNotExist) && !observation.exists {
			continue
		}
		if err != nil {
			return time.Time{}, fmt.Errorf("validate %s: %w", observation.path, err)
		}
		if !observation.exists || !sameFileInfo(observation.info, info) {
			return time.Time{}, errors.New("repository file changed during scan")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(observation.path)
			if err != nil {
				return time.Time{}, fmt.Errorf("validate symlink %s: %w", observation.path, err)
			}
			if target != observation.symlinkTarget {
				return time.Time{}, errors.New("repository symlink changed during scan")
			}
		}
	}
	return observedAt, nil
}

func sameFileInfo(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Size() == after.Size() &&
		before.Mode() == after.Mode() && before.ModTime().Equal(after.ModTime())
}

func (s *scanSession) optionalGitValue(ctx context.Context, args ...string) (*string, error) {
	output, present, err := s.runOptionalGit(ctx, args...)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	value := strings.TrimSpace(string(output))
	return &value, nil
}

func (s *scanSession) pathList(ctx context.Context, args ...string) ([]string, error) {
	output, err := s.runGit(ctx, args...)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0)
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

func (s *scanSession) runGit(ctx context.Context, args ...string) ([]byte, error) {
	s.gitCommands++
	return runGit(ctx, s.repository.Root, args...)
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
