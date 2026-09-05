package cue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestRankedCueDeterministicSelectionAndBudget(t *testing.T) {
	state := experimentalState()
	facts := RankedFacts{
		RecentCommits: []string{"Newest change", "Older change"},
		EntryPoints:   []string{"cmd/tool/main.go"},
		MakeTargets:   []string{"test", "build"},
	}
	first, firstTokens, err := RankedOverview(state, facts, "current", 500)
	if err != nil {
		t.Fatal(err)
	}
	second, secondTokens, err := RankedOverview(state, facts, "current", 500)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || firstTokens != secondTokens || firstTokens > 500 {
		t.Fatalf("ranked cue is not deterministic and budgeted: %d %d\n%s\n%s", firstTokens, secondTokens, first, second)
	}
	if !bytes.Contains(first, []byte("README.md")) || !bytes.Contains(first, []byte("ARCHITECTURE.md")) {
		t.Fatalf("ranked cue omitted high-value documents: %s", first)
	}
	if bytes.Contains(first, []byte("repository_digest")) || bytes.Contains(first, []byte("status_digest")) {
		t.Fatalf("ranked cue exposed provenance digests: %s", first)
	}
}

func TestRankedCuePrioritizesRootDocumentsAndDepthOneDirectories(t *testing.T) {
	state := rankedSelectionPressureState()
	facts := RankedFacts{
		RecentCommits: []string{"Newest change", "Older change"},
		EntryPoints:   []string{"src/application/main.go"},
		MakeTargets:   []string{"verify", "build"},
	}
	built := buildRankedContent(state, facts)
	if len(built.Documents) != rankedDocumentLimit {
		t.Fatalf("document candidate count = %d, want %d", len(built.Documents), rankedDocumentLimit)
	}
	for index, path := range []string{"README.md", "AGENTS.md", "ARCHITECTURE.md", "CHANGELOG.md"} {
		if built.Documents[index] != path {
			t.Fatalf("document priority at %d = %q, want %q: %#v", index, built.Documents[index], path, built.Documents)
		}
	}

	tests := []struct {
		name      string
		build     func() ([]byte, int, error)
		structure bool
	}{
		{
			name:  "ranked",
			build: func() ([]byte, int, error) { return RankedOverview(state, facts, "current", 500) },
		},
		{
			name: "structural",
			build: func() ([]byte, int, error) {
				return StructuralOverview(state, facts, []StructuralCandidate{{
					Language: "Go", Module: "src/application/main.go", Kind: "function", Name: "main", Signature: "func main()",
				}}, "current", 500)
			},
			structure: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			first, firstTokens, err := test.build()
			if err != nil {
				t.Fatal(err)
			}
			second, secondTokens, err := test.build()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, second) || firstTokens != secondTokens || firstTokens > 500 {
				t.Fatalf("cue is not deterministic and budgeted: %d %d\n%s\n%s", firstTokens, secondTokens, first, second)
			}
			var envelope struct {
				Content rankedContent `json:"content"`
			}
			if err := json.Unmarshal(first, &envelope); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"README.md", "AGENTS.md", "ARCHITECTURE.md", "CHANGELOG.md"} {
				if !containsString(envelope.Content.Documents, path) {
					t.Fatalf("cue omitted root document %q: %s", path, first)
				}
			}
			for _, directory := range []string{"bin/", "configure/", "docs/", "src/", "tests/"} {
				if !containsString(envelope.Content.Directories, directory) {
					t.Fatalf("cue omitted depth-one directory %q: %s", directory, first)
				}
			}
			if containsString(envelope.Content.Directories, "tests/integration/") {
				t.Fatalf("cue retained the lowest-ranked depth-two directory under pressure: %s", first)
			}
			if !containsString(envelope.Content.EntryPoints, "src/application/main.go") {
				t.Fatalf("cue omitted entry point: %s", first)
			}
			if test.structure && len(envelope.Content.Structure) == 0 {
				t.Fatalf("structural cue omitted structural candidate: %s", first)
			}
		})
	}
}

func TestPlaceboCueContainsOnlyMinimalRepositoryState(t *testing.T) {
	serialized, tokens, err := Placebo(experimentalState(), "current", 500)
	if err != nil {
		t.Fatal(err)
	}
	if tokens > 500 || !bytes.Contains(serialized, []byte(`"kind":"placebo"`)) {
		t.Fatalf("unexpected placebo cue: %s", serialized)
	}
	for _, forbidden := range []string{"repository_digest", "status_digest", "repository_id", "documents"} {
		if bytes.Contains(serialized, []byte(forbidden)) {
			t.Fatalf("placebo cue contains %q: %s", forbidden, serialized)
		}
	}
}

func TestProvenancePathFilterAndDeltaV2OmitContentDigests(t *testing.T) {
	state := experimentalState()
	serialized, _, err := Provenance(state, "cmd", "current", 500)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(serialized, []byte("cmd/tool/main.go")) || bytes.Contains(serialized, []byte("README.md")) {
		t.Fatalf("unexpected provenance filter: %s", serialized)
	}
	before := model.File{Path: "README.md", ContentDigest: "sha256:before", SizeBytes: 90}
	after := before
	after.ContentDigest = "sha256:after"
	after.SizeBytes = 100
	after.WorkingTreeMode = "100755"
	before.WorkingTreeMode = "100644"
	delta, _, err := DeltaV2(state, model.Snapshot{ID: "snapshot-000000"}, []model.DeltaItem{{
		Operation: "file.content_changed", Entity: "file:README.md", Path: "README.md", Before: &before, After: &after,
	}}, "current", 500)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(delta), "sha256:before") || strings.Contains(string(delta), "sha256:after") {
		t.Fatalf("delta v2 exposed content digests: %s", delta)
	}
	if !strings.Contains(string(delta), `"working_tree_mode":"100755"`) {
		t.Fatalf("delta v2 omitted concurrent metadata: %s", delta)
	}
}

func TestProvenanceReportsBudgetTruncation(t *testing.T) {
	state := experimentalState()
	state.Files = make([]model.File, 0, 100)
	for index := range 100 {
		state.Files = append(state.Files, model.File{
			Path: fmt.Sprintf("src/path-%03d.go", index), Exists: true, SizeBytes: 100,
			ContentDigest: "sha256:" + strings.Repeat("a", 64), IndexObject: strings.Repeat("b", 40),
		})
	}
	serialized, estimated, err := Provenance(state, "", "current", 500)
	if err != nil {
		t.Fatal(err)
	}
	if estimated > 500 {
		t.Fatalf("provenance cue exceeded budget: %d", estimated)
	}
	var envelope struct {
		Content  provenanceContent `json:"content"`
		Warnings []Warning         `json:"warnings"`
	}
	if err := json.Unmarshal(serialized, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Content.MatchedFiles != 100 || len(envelope.Content.Files) >= envelope.Content.MatchedFiles {
		t.Fatalf("provenance cue did not expose truncation totals: %#v", envelope.Content)
	}
	if len(envelope.Warnings) != 1 || envelope.Warnings[0].Code != "provenance_files_omitted" ||
		envelope.Warnings[0].Count != envelope.Content.MatchedFiles-len(envelope.Content.Files) {
		t.Fatalf("provenance cue has incorrect truncation warning: %#v", envelope.Warnings)
	}
}

func TestStructuralContextBudget(t *testing.T) {
	candidates := make([]StructuralCandidate, 0, 100)
	for index := range 100 {
		candidates = append(candidates, StructuralCandidate{Language: "Go", Module: "internal/example.go", Kind: "function", Name: strings.Repeat("name", index+1)})
	}
	serialized, estimated, err := StructuralOverview(experimentalState(), RankedFacts{}, candidates, "current", 500)
	if err != nil {
		t.Fatal(err)
	}
	if estimated > 500 {
		t.Fatalf("structural cue exceeded budget: %d\n%s", estimated, serialized)
	}
	var document map[string]any
	if err := json.Unmarshal(serialized, &document); err != nil {
		t.Fatal(err)
	}
}

func TestExperimentalCueOutputsValidateAgainstSchema(t *testing.T) {
	state := experimentalState()
	facts := RankedFacts{EntryPoints: []string{"cmd/tool/main.go"}}
	candidates := []StructuralCandidate{{Language: "Go", Module: "cmd/tool/main.go", Kind: "function", Name: "main", Signature: "func main()"}}
	outputs := make([][]byte, 0, 5)
	builders := []func() ([]byte, int, error){
		func() ([]byte, int, error) { return Placebo(state, "current", 500) },
		func() ([]byte, int, error) { return RankedOverview(state, facts, "current", 500) },
		func() ([]byte, int, error) { return StructuralOverview(state, facts, candidates, "current", 500) },
		func() ([]byte, int, error) { return Provenance(state, "", "current", 500) },
		func() ([]byte, int, error) {
			return DeltaV2(state, model.Snapshot{ID: "snapshot-000000"}, nil, "current", 500)
		},
	}
	for _, build := range builders {
		serialized, _, err := build()
		if err != nil {
			t.Fatal(err)
		}
		outputs = append(outputs, serialized)
	}
	path := filepath.Join("..", "..", "docs", "schema", "cue-v2.schema.json")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var schemaDocument any
	if err := json.NewDecoder(file).Decode(&schemaDocument); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource("cue-v2.schema.json", schemaDocument); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("cue-v2.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, serialized := range outputs {
		var document any
		if err := json.Unmarshal(serialized, &document); err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(document); err != nil {
			t.Fatalf("cue v2 validation failed: %v\n%s", err, serialized)
		}
	}
}

func experimentalState() model.CurrentState {
	branch := "main"
	head := "0123456789abcdef"
	return model.CurrentState{
		Repository: model.Repository{ID: "repo-test", Name: "test"}, Epoch: model.Epoch{ID: "epoch-000001"},
		Snapshot: model.Snapshot{
			ID: "snapshot-000001", RepositoryDigest: "sha256:repository",
			Basis: model.Basis{Branch: &branch, Head: &head, StatusDigest: "sha256:status", ObservedAt: time.Date(2026, 8, 28, 12, 1, 2, 123, time.UTC)},
		},
		Files: []model.File{
			{Path: "docs/notes.md", Exists: true, SizeBytes: 20, Document: true, ContentDigest: "sha256:notes"},
			{Path: "README.md", Exists: true, SizeBytes: 100, Document: true, ContentDigest: "sha256:readme"},
			{Path: "docs/ARCHITECTURE.md", Exists: true, SizeBytes: 80, Document: true, ContentDigest: "sha256:architecture"},
			{Path: "cmd/tool/main.go", Exists: true, SizeBytes: 200, Language: "Go", ContentDigest: "sha256:main"},
		},
	}
}

func rankedSelectionPressureState() model.CurrentState {
	state := experimentalState()
	state.Files = []model.File{
		{Path: "README.md", Exists: true, SizeBytes: 100, Document: true},
		{Path: "AGENTS.md", Exists: true, SizeBytes: 100, Document: true},
		{Path: "ARCHITECTURE.md", Exists: true, SizeBytes: 100, Document: true},
		{Path: "CHANGELOG.md", Exists: true, SizeBytes: 100, Document: true},
		{Path: "bin/tool", Exists: true, SizeBytes: 100},
		{Path: "configure/CONFIG", Exists: true, SizeBytes: 100},
		{Path: "src/application/main.go", Exists: true, SizeBytes: 100, Language: "Go"},
		{Path: "tests/integration/main_test.go", Exists: true, SizeBytes: 100, Language: "Go"},
		{Path: "docs/README.md", Exists: true, SizeBytes: 100, Document: true},
		{Path: "docs/guides/README.md", Exists: true, SizeBytes: 100, Document: true},
	}
	for index := range 20 {
		state.Files = append(state.Files, model.File{
			Path:   fmt.Sprintf("docs/notes/note-%02d-%s.md", index, strings.Repeat("selection-pressure-", 10)),
			Exists: true, SizeBytes: 100, Document: true,
		})
	}
	return state
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
