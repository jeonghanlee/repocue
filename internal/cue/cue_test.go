package cue

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestOverviewIsDeterministicAndBudgeted(t *testing.T) {
	branch := "main"
	head := "abc123"
	observed := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	state := model.CurrentState{
		Repository: model.Repository{ID: "repo-test", Name: "test"},
		Epoch:      model.Epoch{ID: "epoch-000001"},
		Snapshot: model.Snapshot{
			ID:               "snapshot-000001",
			Basis:            model.Basis{Branch: &branch, Head: &head, StatusDigest: "sha256:status", ObservedAt: observed},
			RepositoryDigest: "sha256:repository",
		},
		Files: []model.File{
			{Path: "README.md", Exists: true, SizeBytes: 100, Language: "Markdown", Document: true},
			{Path: "cmd/tool/main.go", Exists: true, SizeBytes: 200, Language: "Go"},
		},
	}
	first, firstEstimate, err := Overview(state, "current", 500)
	if err != nil {
		t.Fatal(err)
	}
	second, secondEstimate, err := Overview(state, "current", 500)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("overview is not deterministic:\n%s\n%s", first, second)
	}
	if firstEstimate != secondEstimate || firstEstimate > 500 {
		t.Fatalf("unexpected estimates: %d and %d", firstEstimate, secondEstimate)
	}
}

func TestDeltaProjectsOnlyChangedMetadata(t *testing.T) {
	state := model.CurrentState{
		Repository: model.Repository{ID: "repo-test", Name: "test"},
		Epoch:      model.Epoch{ID: "epoch-000001"},
		Snapshot:   model.Snapshot{ID: "snapshot-000002", Basis: model.Basis{ObservedAt: time.Unix(0, 0).UTC()}},
	}
	before := model.File{EntityID: "file:a", Path: "a", Exists: true, WorkingTreeMode: "100644", ContentDigest: "sha256:same"}
	after := before
	after.WorkingTreeMode = "100755"
	item := model.DeltaItem{Operation: "file.metadata_changed", Entity: "file:a", Path: "a", Before: &before, After: &after}
	serialized, _, err := Delta(state, model.Snapshot{ID: "snapshot-000001"}, []model.DeltaItem{item}, "current", 500)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Content struct {
			Changes []DeltaChange `json:"changes"`
		} `json:"content"`
	}
	if err := json.Unmarshal(serialized, &envelope); err != nil {
		t.Fatal(err)
	}
	change := envelope.Content.Changes[0]
	beforeValues := change.Before.(map[string]any)
	if len(beforeValues) != 1 || beforeValues["working_tree_mode"] != "100644" {
		t.Fatalf("unexpected compact metadata: %#v", beforeValues)
	}
}

func TestOverviewRejectsInsufficientBudget(t *testing.T) {
	state := model.CurrentState{
		Repository: model.Repository{ID: "repo-test", Name: "test"},
		Epoch:      model.Epoch{ID: "epoch-000001"},
		Snapshot:   model.Snapshot{ID: "snapshot-000001", Basis: model.Basis{ObservedAt: time.Unix(0, 0).UTC()}},
	}
	_, _, err := Overview(state, "current", 1)
	if !errors.Is(err, ErrBudgetTooSmall) {
		t.Fatalf("got %v, want ErrBudgetTooSmall", err)
	}
}

func TestCueOutputsValidateAgainstJSONSchema(t *testing.T) {
	branch := "main"
	head := "abc123"
	state := model.CurrentState{
		Repository: model.Repository{ID: "repo-test", Name: "test"},
		Epoch:      model.Epoch{ID: "epoch-000001"},
		Snapshot: model.Snapshot{
			ID: "snapshot-000002",
			Basis: model.Basis{
				Branch: &branch, Head: &head, StatusDigest: "sha256:status",
				ObservedAt: time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
			},
			RepositoryDigest: "sha256:repository",
		},
		Files: []model.File{
			{Path: "README.md", Exists: true, SizeBytes: 100, Language: "Markdown", Document: true},
			{Path: "cmd/tool/main.go", Exists: true, SizeBytes: 200, Language: "Go"},
		},
	}
	schema := compileCueSchema(t)
	for budget := 1; budget <= 500; budget++ {
		if serialized, _, err := Overview(state, "current", budget); err == nil {
			validateCue(t, schema, serialized)
		}
		items := []model.DeltaItem{{
			Operation: "file.content_changed", Entity: "file:README.md", Path: "README.md",
			Before: &model.File{ContentDigest: "sha256:before", SizeBytes: 90},
			After:  &model.File{ContentDigest: "sha256:after", SizeBytes: 100},
		}}
		if serialized, _, err := Delta(state, model.Snapshot{ID: "snapshot-000001"}, items, "current", budget); err == nil {
			validateCue(t, schema, serialized)
		}
	}
}

func compileCueSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "schema", "cue-v1.schema.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var document any
	if err := json.NewDecoder(file).Decode(&document); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource("cue-v1.schema.json", document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("cue-v1.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func validateCue(t *testing.T, schema *jsonschema.Schema, serialized []byte) {
	t.Helper()
	var document any
	if err := json.Unmarshal(serialized, &document); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("cue does not validate: %v\n%s", err, serialized)
	}
}
