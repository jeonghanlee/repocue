package cue

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jeonghanlee/repocue/internal/metrics"
	"github.com/jeonghanlee/repocue/internal/model"
)

const (
	ExperimentalSchemaVersion = "repocue/cue-2"
	rankedDocumentLimit       = 12
)

type RankedFacts struct {
	RecentCommits []string `json:"recent_commits"`
	EntryPoints   []string `json:"entry_points"`
	MakeTargets   []string `json:"make_targets"`
}

type StructuralCandidate struct {
	Language  string `json:"language"`
	Module    string `json:"module"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Signature string `json:"signature,omitempty"`
}

type experimentalRepository struct {
	Branch *string `json:"branch"`
	Head   *string `json:"head"`
	Dirty  bool    `json:"dirty"`
}

type experimentalEnvelope struct {
	SchemaVersion string                 `json:"schema_version"`
	Kind          string                 `json:"kind"`
	Repository    experimentalRepository `json:"repository"`
	Epoch         string                 `json:"epoch,omitempty"`
	Snapshot      string                 `json:"snapshot"`
	Freshness     string                 `json:"freshness"`
	ObservedAt    string                 `json:"observed_at"`
	Budget        Budget                 `json:"budget"`
	Content       any                    `json:"content"`
	Warnings      []Warning              `json:"warnings"`
}

type rankedContent struct {
	TrackedFiles  int                   `json:"tracked_files"`
	PresentFiles  int                   `json:"present_files"`
	Bytes         int64                 `json:"bytes"`
	Documents     []string              `json:"documents,omitempty"`
	EntryPoints   []string              `json:"entry_points,omitempty"`
	MakeTargets   []string              `json:"make_targets,omitempty"`
	RecentCommits []string              `json:"recent_commits,omitempty"`
	Directories   []string              `json:"directories,omitempty"`
	Structure     []StructuralCandidate `json:"structure,omitempty"`
}

type provenanceContent struct {
	RepositoryID     string           `json:"repository_id"`
	RepositoryDigest string           `json:"repository_digest"`
	StatusDigest     string           `json:"status_digest"`
	MatchedFiles     int              `json:"matched_files"`
	Files            []provenanceFile `json:"files"`
}

type provenanceFile struct {
	Path          string `json:"path"`
	ContentDigest string `json:"content_digest"`
	IndexObject   string `json:"index_object"`
	SizeBytes     int64  `json:"size_bytes"`
}

func Placebo(state model.CurrentState, freshness string, maxTokens int) ([]byte, int, error) {
	envelope := experimentalBase(state, "placebo", freshness, maxTokens)
	envelope.Epoch = ""
	envelope.Content = struct{}{}
	return marshalExperimental(envelope, maxTokens)
}

func RankedOverview(state model.CurrentState, facts RankedFacts, freshness string, maxTokens int) ([]byte, int, error) {
	content := buildRankedContent(state, facts)
	return fitRanked(state, "ranked", freshness, maxTokens, &content)
}

func StructuralOverview(state model.CurrentState, facts RankedFacts, candidates []StructuralCandidate, freshness string, maxTokens int) ([]byte, int, error) {
	content := buildRankedContent(state, facts)
	content.Structure = append([]StructuralCandidate{}, candidates...)
	sort.Slice(content.Structure, func(i, j int) bool {
		left, right := content.Structure[i], content.Structure[j]
		if left.Module != right.Module {
			return left.Module < right.Module
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.Signature < right.Signature
	})
	return fitRanked(state, "structural-oracle", freshness, maxTokens, &content)
}

func Provenance(state model.CurrentState, pathPrefix, freshness string, maxTokens int) ([]byte, int, error) {
	content := provenanceContent{
		RepositoryID: state.Repository.ID, RepositoryDigest: state.Snapshot.RepositoryDigest,
		StatusDigest: state.Snapshot.Basis.StatusDigest, Files: []provenanceFile{},
	}
	for _, file := range state.Files {
		if pathPrefix != "" && file.Path != pathPrefix && !strings.HasPrefix(file.Path, strings.TrimSuffix(pathPrefix, "/")+"/") {
			continue
		}
		content.Files = append(content.Files, provenanceFile{
			Path: file.Path, ContentDigest: file.ContentDigest, IndexObject: file.IndexObject, SizeBytes: file.SizeBytes,
		})
	}
	sort.Slice(content.Files, func(i, j int) bool { return content.Files[i].Path < content.Files[j].Path })
	content.MatchedFiles = len(content.Files)
	envelope := experimentalBase(state, "provenance", freshness, maxTokens)
	envelope.Content = &content
	for {
		omitted := content.MatchedFiles - len(content.Files)
		if omitted > 0 {
			envelope.Warnings = []Warning{{Code: "provenance_files_omitted", Count: omitted}}
		}
		serialized, estimated, err := marshalExperimental(envelope, maxTokens)
		if err == nil {
			return serialized, estimated, nil
		}
		if len(content.Files) == 0 {
			return nil, estimated, err
		}
		content.Files = content.Files[:len(content.Files)-1]
		envelope.Content = &content
	}
}

func DeltaV2(current model.CurrentState, from model.Snapshot, items []model.DeltaItem, freshness string, maxTokens int) ([]byte, int, error) {
	changes := compactChangesV2(items)
	content := &DeltaContent{FromSnapshot: from.ID, ToSnapshot: current.Snapshot.ID, ChangeCount: len(items), Changes: changes}
	envelope := experimentalBase(current, "delta", freshness, maxTokens)
	envelope.Content = content
	for {
		serialized, estimated, err := marshalExperimental(envelope, maxTokens)
		if err == nil {
			return serialized, estimated, nil
		}
		if len(content.Changes) == 0 {
			return nil, estimated, err
		}
		content.Changes = content.Changes[:len(content.Changes)-1]
	}
}

func fitRanked(state model.CurrentState, kind, freshness string, maxTokens int, content *rankedContent) ([]byte, int, error) {
	envelope := experimentalBase(state, kind, freshness, maxTokens)
	envelope.Content = content
	for {
		serialized, estimated, err := marshalExperimental(envelope, maxTokens)
		if err == nil {
			return serialized, estimated, nil
		}
		switch {
		case hasDepthTwoDirectory(content.Directories):
			content.Directories = content.Directories[:len(content.Directories)-1]
		case len(content.Documents) > 0:
			content.Documents = content.Documents[:len(content.Documents)-1]
		case len(content.MakeTargets) > 0:
			content.MakeTargets = content.MakeTargets[:len(content.MakeTargets)-1]
		case len(content.RecentCommits) > 0:
			content.RecentCommits = content.RecentCommits[:len(content.RecentCommits)-1]
		case len(content.Directories) > 0:
			content.Directories = content.Directories[:len(content.Directories)-1]
		case len(content.EntryPoints) > 0:
			content.EntryPoints = content.EntryPoints[:len(content.EntryPoints)-1]
		case len(content.Structure) > 0:
			content.Structure = content.Structure[:len(content.Structure)-1]
		default:
			return nil, estimated, err
		}
	}
}

func hasDepthTwoDirectory(directories []string) bool {
	return len(directories) > 0 && strings.Count(directories[len(directories)-1], "/") > 1
}

func buildRankedContent(state model.CurrentState, facts RankedFacts) rankedContent {
	content := rankedContent{
		TrackedFiles: len(state.Files), Documents: []string{}, EntryPoints: sortedUniqueStrings(facts.EntryPoints),
		MakeTargets: sortedUniqueStrings(facts.MakeTargets), RecentCommits: append([]string{}, facts.RecentCommits...),
		Directories: []string{}, Structure: []StructuralCandidate{},
	}
	depthOneDirectories := map[string]struct{}{}
	depthTwoDirectories := map[string]struct{}{}
	for _, file := range state.Files {
		if file.Exists {
			content.PresentFiles++
			content.Bytes += file.SizeBytes
		}
		if file.Document && file.Exists {
			content.Documents = append(content.Documents, file.Path)
		}
		parts := strings.Split(filepath.ToSlash(file.Path), "/")
		if len(parts) > 1 {
			depthOneDirectories[parts[0]+"/"] = struct{}{}
			if len(parts) > 2 {
				depthTwoDirectories[strings.Join(parts[:2], "/")+"/"] = struct{}{}
			}
		}
	}
	for directory := range depthOneDirectories {
		content.Directories = append(content.Directories, directory)
	}
	sort.Strings(content.Directories)
	depthTwo := make([]string, 0, len(depthTwoDirectories))
	for directory := range depthTwoDirectories {
		depthTwo = append(depthTwo, directory)
	}
	sort.Strings(depthTwo)
	content.Directories = append(content.Directories, depthTwo...)
	sort.Slice(content.Documents, func(i, j int) bool {
		left, right := documentRank(content.Documents[i]), documentRank(content.Documents[j])
		if left != right {
			return left < right
		}
		leftDepth := strings.Count(filepath.ToSlash(content.Documents[i]), "/")
		rightDepth := strings.Count(filepath.ToSlash(content.Documents[j]), "/")
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return content.Documents[i] < content.Documents[j]
	})
	if len(content.Documents) > rankedDocumentLimit {
		content.Documents = content.Documents[:rankedDocumentLimit]
	}
	if len(content.RecentCommits) > 5 {
		content.RecentCommits = content.RecentCommits[:5]
	}
	return content
}

func documentRank(path string) int {
	normalized := filepath.ToSlash(path)
	base := strings.ToUpper(filepath.Base(normalized))
	if !strings.Contains(normalized, "/") {
		switch base {
		case "README", "README.MD", "README.RST":
			return 0
		case "AGENTS.MD":
			return 1
		case "ARCHITECTURE.MD":
			return 2
		case "CHANGELOG", "CHANGELOG.MD":
			return 3
		}
	}
	switch base {
	case "README", "README.MD", "README.RST":
		return 10
	case "AGENTS.MD":
		return 11
	case "ARCHITECTURE.MD":
		return 12
	case "CHANGELOG", "CHANGELOG.MD":
		return 13
	default:
		return 100
	}
}

func experimentalBase(state model.CurrentState, kind, freshness string, maxTokens int) experimentalEnvelope {
	return experimentalEnvelope{
		SchemaVersion: ExperimentalSchemaVersion, Kind: kind,
		Repository: experimentalRepository{Branch: state.Snapshot.Basis.Branch, Head: state.Snapshot.Basis.Head, Dirty: state.Snapshot.Basis.Dirty},
		Epoch:      state.Epoch.ID, Snapshot: state.Snapshot.ID, Freshness: freshness,
		ObservedAt: state.Snapshot.Basis.ObservedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Budget:     Budget{MaxTokens: maxTokens}, Warnings: []Warning{},
	}
}

func marshalExperimental(envelope experimentalEnvelope, maxTokens int) ([]byte, int, error) {
	if maxTokens < 1 {
		return nil, 0, fmt.Errorf("%w: maximum must be positive", ErrBudgetTooSmall)
	}
	for range 4 {
		serialized, err := json.Marshal(envelope)
		if err != nil {
			return nil, 0, err
		}
		estimated := metrics.EstimateTokens(serialized)
		if envelope.Budget.EstimatedTokens == estimated {
			if estimated > maxTokens {
				return nil, estimated, fmt.Errorf("%w: minimum estimate is %d tokens", ErrBudgetTooSmall, estimated)
			}
			return serialized, estimated, nil
		}
		envelope.Budget.EstimatedTokens = estimated
	}
	serialized, err := json.Marshal(envelope)
	if err != nil {
		return nil, 0, err
	}
	estimated := metrics.EstimateTokens(serialized)
	if estimated > maxTokens {
		return nil, estimated, fmt.Errorf("%w: minimum estimate is %d tokens", ErrBudgetTooSmall, estimated)
	}
	return serialized, estimated, nil
}

func compactChangesV2(items []model.DeltaItem) []DeltaChange {
	changes := compactChanges(items)
	for index := range changes {
		changes[index].Before = withoutDigest(changes[index].Before)
		changes[index].After = withoutDigest(changes[index].After)
	}
	return changes
}

func withoutDigest(value any) any {
	values, ok := value.(map[string]any)
	if !ok {
		return value
	}
	result := make(map[string]any, len(values))
	for key, item := range values {
		if key != "digest" {
			result[key] = item
		}
	}
	return result
}

func sortedUniqueStrings(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	if len(result) == 0 {
		return []string{}
	}
	write := 1
	for read := 1; read < len(result); read++ {
		if result[read] != result[write-1] {
			result[write] = result[read]
			write++
		}
	}
	return result[:write]
}
