package snapshot

import (
	"testing"

	"github.com/jeonghanlee/repocue/internal/model"
)

func TestDiffFileOperations(t *testing.T) {
	tests := []struct {
		name      string
		before    []model.File
		after     []model.File
		operation string
	}{
		{
			name:      "added",
			after:     []model.File{{EntityID: "file:a.go", Path: "a.go", Exists: true}},
			operation: "file.added",
		},
		{
			name:      "content changed",
			before:    []model.File{{EntityID: "file:a.go", Path: "a.go", Exists: true, ContentDigest: "sha256:1"}},
			after:     []model.File{{EntityID: "file:a.go", Path: "a.go", Exists: true, ContentDigest: "sha256:2"}},
			operation: "file.content_changed",
		},
		{
			name:      "deleted from working tree",
			before:    []model.File{{EntityID: "file:a.go", Path: "a.go", Exists: true}},
			after:     []model.File{{EntityID: "file:a.go", Path: "a.go", Exists: false}},
			operation: "file.deleted",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := Diff("repo-test", model.Snapshot{}, model.Snapshot{}, test.before, test.after)
			if len(items) != 1 {
				t.Fatalf("got %d items, want 1: %#v", len(items), items)
			}
			if items[0].Operation != test.operation {
				t.Fatalf("got operation %q, want %q", items[0].Operation, test.operation)
			}
		})
	}
}
