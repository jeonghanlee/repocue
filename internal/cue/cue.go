package cue

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jeonghanlee/repocue/internal/metrics"
	"github.com/jeonghanlee/repocue/internal/model"
)

var ErrBudgetTooSmall = errors.New("token budget is too small for required provenance")

type Budget struct {
	MaxTokens       int `json:"max_tokens"`
	EstimatedTokens int `json:"estimated_tokens"`
}

type Repository struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Branch *string `json:"branch"`
	Head   *string `json:"head"`
	Dirty  bool    `json:"dirty"`
}

type Basis struct {
	StatusDigest     string `json:"status_digest"`
	RepositoryDigest string `json:"repository_digest"`
	ObservedAt       string `json:"observed_at"`
}

type Warning struct {
	Code  string `json:"code"`
	Count int    `json:"count,omitempty"`
}

type More struct {
	View      string `json:"view"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

type Language struct {
	Name  string `json:"name"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
}

type OverviewContent struct {
	TrackedFiles int        `json:"tracked_files"`
	PresentFiles int        `json:"present_files"`
	Bytes        int64      `json:"bytes"`
	Documents    []string   `json:"documents,omitempty"`
	Languages    []Language `json:"languages,omitempty"`
	TopLevel     []string   `json:"top_level,omitempty"`
}

type DeltaContent struct {
	FromSnapshot string        `json:"from_snapshot"`
	ToSnapshot   string        `json:"to_snapshot"`
	ChangeCount  int           `json:"change_count"`
	Changes      []DeltaChange `json:"changes"`
}

type DeltaChange struct {
	Operation string `json:"op"`
	Entity    string `json:"entity"`
	Path      string `json:"path,omitempty"`
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
}

type Envelope struct {
	SchemaVersion string     `json:"schema_version"`
	Kind          string     `json:"kind"`
	Repository    Repository `json:"repository"`
	Epoch         string     `json:"epoch"`
	Snapshot      string     `json:"snapshot"`
	Basis         Basis      `json:"basis"`
	Freshness     string     `json:"freshness"`
	Budget        Budget     `json:"budget"`
	Content       any        `json:"content"`
	Warnings      []Warning  `json:"warnings"`
	More          []More     `json:"more"`
}

func Overview(state model.CurrentState, freshness string, maxTokens int) ([]byte, int, error) {
	content := summarize(state.Files)
	envelope := baseEnvelope(state, freshness, maxTokens)
	envelope.Kind = "overview"
	envelope.Content = &content
	if len(state.Snapshot.Basis.Untracked) > 0 {
		envelope.Warnings = append(envelope.Warnings, Warning{Code: "untracked_not_indexed", Count: len(state.Snapshot.Basis.Untracked)})
	}
	moreRemoved := false
	for {
		serialized, estimated, err := marshal(envelope)
		if err != nil {
			return nil, 0, err
		}
		if estimated <= maxTokens {
			return serialized, estimated, nil
		}
		switch {
		case len(content.Documents) > 0:
			content.Documents = content.Documents[:len(content.Documents)-1]
		case len(content.TopLevel) > 0:
			content.TopLevel = content.TopLevel[:len(content.TopLevel)-1]
		case len(content.Languages) > 0:
			content.Languages = content.Languages[:len(content.Languages)-1]
		case len(envelope.More) > 0:
			envelope.More = []More{}
			moreRemoved = true
		default:
			return nil, estimated, fmt.Errorf("%w: minimum estimate is %d tokens", ErrBudgetTooSmall, estimated)
		}
		if !moreRemoved && len(envelope.More) == 0 {
			envelope.More = []More{{View: "overview", MaxTokens: maxTokens * 2}}
		}
	}
}

func Delta(current model.CurrentState, from model.Snapshot, items []model.DeltaItem, freshness string, maxTokens int) ([]byte, int, error) {
	envelope := baseEnvelope(current, freshness, maxTokens)
	envelope.Kind = "delta"
	content := &DeltaContent{
		FromSnapshot: from.ID,
		ToSnapshot:   current.Snapshot.ID,
		ChangeCount:  len(items),
		Changes:      compactChanges(items),
	}
	envelope.Content = content
	for {
		serialized, estimated, err := marshal(envelope)
		if err != nil {
			return nil, 0, err
		}
		if estimated <= maxTokens {
			return serialized, estimated, nil
		}
		if len(content.Changes) == 0 && len(envelope.More) > 0 {
			envelope.More = []More{}
			continue
		}
		if len(content.Changes) == 0 {
			return nil, estimated, fmt.Errorf("%w: minimum estimate is %d tokens", ErrBudgetTooSmall, estimated)
		}
		content.Changes = content.Changes[:len(content.Changes)-1]
		if len(envelope.More) == 0 {
			envelope.More = []More{{View: "delta", MaxTokens: maxTokens * 2}}
		}
	}
}

func compactChanges(items []model.DeltaItem) []DeltaChange {
	changes := make([]DeltaChange, 0, len(items))
	for _, item := range items {
		change := DeltaChange{Operation: item.Operation, Entity: item.Entity, Path: item.Path}
		before, beforeIsFile := item.Before.(*model.File)
		after, afterIsFile := item.After.(*model.File)
		if !beforeIsFile && !afterIsFile {
			change.Before = item.Before
			change.After = item.After
			changes = append(changes, change)
			continue
		}
		switch item.Operation {
		case "file.added":
			change.After = fileSummary(after)
		case "file.deleted":
			change.Before = fileSummary(before)
		case "file.restored":
			change.Before = map[string]any{"exists": false}
			change.After = fileSummary(after)
		case "file.content_changed":
			beforeSummary := contentSummary(before)
			afterSummary := contentSummary(after)
			beforeMetadata, afterMetadata := changedMetadata(before, after)
			mergeSummary(beforeSummary, beforeMetadata)
			mergeSummary(afterSummary, afterMetadata)
			change.Before = beforeSummary
			change.After = afterSummary
		case "file.metadata_changed":
			change.Before, change.After = changedMetadata(before, after)
		case "file.state_changed":
			change.Before = before.WorkingState
			change.After = after.WorkingState
		default:
			change.Before = item.Before
			change.After = item.After
		}
		changes = append(changes, change)
	}
	return changes
}

func fileSummary(file *model.File) map[string]any {
	if file == nil {
		return nil
	}
	return map[string]any{
		"digest":        file.ContentDigest,
		"size_bytes":    file.SizeBytes,
		"file_type":     file.FileType,
		"language":      file.Language,
		"document":      file.Document,
		"working_state": file.WorkingState,
	}
}

func contentSummary(file *model.File) map[string]any {
	if file == nil {
		return nil
	}
	return map[string]any{"digest": file.ContentDigest, "size_bytes": file.SizeBytes}
}

func changedMetadata(before, after *model.File) (map[string]any, map[string]any) {
	oldValues := map[string]any{}
	newValues := map[string]any{}
	if before == nil || after == nil {
		return oldValues, newValues
	}
	addChanged(oldValues, newValues, "index_mode", before.IndexMode, after.IndexMode)
	addChanged(oldValues, newValues, "index_object", before.IndexObject, after.IndexObject)
	addChanged(oldValues, newValues, "working_tree_mode", before.WorkingTreeMode, after.WorkingTreeMode)
	addChanged(oldValues, newValues, "file_type", before.FileType, after.FileType)
	addChanged(oldValues, newValues, "language", before.Language, after.Language)
	addChanged(oldValues, newValues, "document", before.Document, after.Document)
	addChanged(oldValues, newValues, "working_state", before.WorkingState, after.WorkingState)
	return oldValues, newValues
}

func addChanged(before, after map[string]any, key string, oldValue, newValue any) {
	if oldValue == newValue {
		return
	}
	before[key] = oldValue
	after[key] = newValue
}

func mergeSummary(summary, changed map[string]any) {
	for key, value := range changed {
		summary[key] = value
	}
}

func baseEnvelope(state model.CurrentState, freshness string, maxTokens int) Envelope {
	return Envelope{
		SchemaVersion: model.SchemaVersion,
		Repository: Repository{
			ID: state.Repository.ID, Name: state.Repository.Name,
			Branch: state.Snapshot.Basis.Branch, Head: state.Snapshot.Basis.Head,
			Dirty: state.Snapshot.Basis.Dirty,
		},
		Epoch:     state.Epoch.ID,
		Snapshot:  state.Snapshot.ID,
		Freshness: freshness,
		Basis: Basis{
			StatusDigest: state.Snapshot.Basis.StatusDigest, RepositoryDigest: state.Snapshot.RepositoryDigest,
			ObservedAt: state.Snapshot.Basis.ObservedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
		},
		Budget:   Budget{MaxTokens: maxTokens},
		Warnings: []Warning{},
		More:     []More{},
	}
}

func summarize(files []model.File) OverviewContent {
	content := OverviewContent{TrackedFiles: len(files), Documents: []string{}, Languages: []Language{}, TopLevel: []string{}}
	type languageCount struct {
		files int
		bytes int64
	}
	languages := map[string]languageCount{}
	topLevel := map[string]struct{}{}
	for _, file := range files {
		if file.Exists {
			content.PresentFiles++
			content.Bytes += file.SizeBytes
		}
		if file.Document && file.Exists {
			content.Documents = append(content.Documents, file.Path)
		}
		if file.Language != "" && file.Exists {
			count := languages[file.Language]
			count.files++
			count.bytes += file.SizeBytes
			languages[file.Language] = count
		}
		first, _, found := strings.Cut(filepath.ToSlash(file.Path), "/")
		if !found {
			first = file.Path
		}
		topLevel[first] = struct{}{}
	}
	for name, count := range languages {
		content.Languages = append(content.Languages, Language{Name: name, Files: count.files, Bytes: count.bytes})
	}
	for path := range topLevel {
		content.TopLevel = append(content.TopLevel, path)
	}
	sort.Strings(content.Documents)
	sort.Strings(content.TopLevel)
	sort.Slice(content.Languages, func(i, j int) bool {
		if content.Languages[i].Bytes == content.Languages[j].Bytes {
			return content.Languages[i].Name < content.Languages[j].Name
		}
		return content.Languages[i].Bytes > content.Languages[j].Bytes
	})
	return content
}

func marshal(envelope Envelope) ([]byte, int, error) {
	for range 4 {
		serialized, err := json.Marshal(envelope)
		if err != nil {
			return nil, 0, err
		}
		estimated := metrics.EstimateTokens(serialized)
		if envelope.Budget.EstimatedTokens == estimated {
			return serialized, estimated, nil
		}
		envelope.Budget.EstimatedTokens = estimated
	}
	serialized, err := json.Marshal(envelope)
	if err != nil {
		return nil, 0, err
	}
	return serialized, metrics.EstimateTokens(serialized), nil
}
