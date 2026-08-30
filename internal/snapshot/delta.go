package snapshot

import (
	"reflect"
	"sort"

	"github.com/jeonghanlee/repocue/internal/model"
)

func Diff(repositoryID string, beforeSnapshot, afterSnapshot model.Snapshot, beforeFiles, afterFiles []model.File) []model.DeltaItem {
	items := repositoryChanges(repositoryID, beforeSnapshot.Basis, afterSnapshot.Basis)
	beforeByPath := indexFiles(beforeFiles)
	afterByPath := indexFiles(afterFiles)
	paths := make([]string, 0, len(beforeByPath)+len(afterByPath))
	seen := make(map[string]struct{})
	for path := range beforeByPath {
		paths = append(paths, path)
		seen[path] = struct{}{}
	}
	for path := range afterByPath {
		if _, found := seen[path]; !found {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		before, hadBefore := beforeByPath[path]
		after, hasAfter := afterByPath[path]
		operation := fileOperation(before, hadBefore, after, hasAfter)
		if operation == "" {
			continue
		}
		item := model.DeltaItem{Operation: operation, Entity: "file:" + path, Path: path}
		if hadBefore {
			value := before
			item.Before = &value
		}
		if hasAfter {
			value := after
			item.After = &value
		}
		items = append(items, item)
	}
	return items
}

func ChangedFileCount(items []model.DeltaItem) int {
	count := 0
	for _, item := range items {
		if item.Path != "" {
			count++
		}
	}
	return count
}

func repositoryChanges(repositoryID string, before, after model.Basis) []model.DeltaItem {
	items := make([]model.DeltaItem, 0, 3)
	if !reflect.DeepEqual(before.Branch, after.Branch) {
		items = append(items, model.DeltaItem{
			Operation: "repository.branch_changed", Entity: repositoryID,
			Before: before.Branch, After: after.Branch,
		})
	}
	if !reflect.DeepEqual(before.Head, after.Head) {
		items = append(items, model.DeltaItem{
			Operation: "repository.head_changed", Entity: repositoryID,
			Before: before.Head, After: after.Head,
		})
	}
	if before.WorkingTreeDigest != after.WorkingTreeDigest {
		items = append(items, model.DeltaItem{
			Operation: "repository.working_tree_changed", Entity: repositoryID,
			Before: before.WorkingTreeDigest, After: after.WorkingTreeDigest,
		})
	}
	return items
}

func indexFiles(files []model.File) map[string]model.File {
	result := make(map[string]model.File, len(files))
	for _, file := range files {
		result[file.Path] = file
	}
	return result
}

func fileOperation(before model.File, hadBefore bool, after model.File, hasAfter bool) string {
	if !hadBefore && hasAfter {
		return "file.added"
	}
	if hadBefore && !hasAfter {
		return "file.deleted"
	}
	if before.Exists && !after.Exists {
		return "file.deleted"
	}
	if !before.Exists && after.Exists {
		return "file.restored"
	}
	if before.ContentDigest != after.ContentDigest || before.SizeBytes != after.SizeBytes {
		return "file.content_changed"
	}
	if before.IndexMode != after.IndexMode || before.IndexObject != after.IndexObject ||
		before.WorkingTreeMode != after.WorkingTreeMode || before.FileType != after.FileType ||
		before.Language != after.Language || before.Document != after.Document {
		return "file.metadata_changed"
	}
	if before.WorkingState != after.WorkingState {
		return "file.state_changed"
	}
	return ""
}
