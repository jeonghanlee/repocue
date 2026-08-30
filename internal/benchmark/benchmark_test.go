package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestParseAndScoreDeterministicFacts(t *testing.T) {
	branch := "main"
	head := "0123456789abcdef"
	answer := Answer{
		SchemaVersion: AnswerSchemaVersion,
		Git: GitState{
			Branch: &branch, Head: &head, Dirty: true,
			TrackedChanges: []string{"b.go", "a.go"}, Untracked: []string{"notes.txt"},
		},
		PrimaryEntryPoints: []EntryPoint{}, MajorComponents: []Component{},
		ImportantDocumentation: []Document{}, RecentRelevantChanges: []string{}, ProjectSymbols: []ProjectSymbol{}, Uncertainties: []string{},
	}
	serialized, err := json.Marshal(answer)
	if err != nil {
		t.Fatal(err)
	}
	_, score, err := ParseAndScore(serialized, model.Basis{
		Branch: &branch, Head: &head, Dirty: true,
		Staged: []string{"a.go"}, Unstaged: []string{"b.go"}, Untracked: []string{"notes.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if score.Passed != score.Total || score.Ratio != 1 {
		t.Fatalf("unexpected score: %#v", score)
	}
}

func TestBenchmarkAnswerValidatesAgainstJSONSchema(t *testing.T) {
	answer := Answer{
		SchemaVersion:      AnswerSchemaVersion,
		Git:                GitState{TrackedChanges: []string{}, Untracked: []string{}},
		PrimaryEntryPoints: []EntryPoint{}, MajorComponents: []Component{},
		ImportantDocumentation: []Document{}, RecentRelevantChanges: []string{}, ProjectSymbols: []ProjectSymbol{}, Uncertainties: []string{},
	}
	path := filepath.Join("..", "..", "docs", "schema", "benchmark-answer-v2.schema.json")
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
	if err := compiler.AddResource("benchmark-answer-v2.schema.json", schemaDocument); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("benchmark-answer-v2.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := json.Marshal(answer)
	if err != nil {
		t.Fatal(err)
	}
	var document any
	if err := json.Unmarshal(serialized, &document); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(document); err != nil {
		t.Fatalf("benchmark answer validation failed: %v", err)
	}
}

func TestAnswerSchemaPropertiesDeclareTypes(t *testing.T) {
	serialized, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", "benchmark-answer-v2.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Type any `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(serialized, &schema); err != nil {
		t.Fatal(err)
	}
	for name, property := range schema.Properties {
		if property.Type == nil {
			t.Errorf("property %q does not declare a type", name)
		}
	}
}

func TestParseAndScoreReportsIncorrectFact(t *testing.T) {
	branch := "main"
	wrong := "feature"
	answer := Answer{
		SchemaVersion:      AnswerSchemaVersion,
		Git:                GitState{Branch: &wrong, TrackedChanges: []string{}, Untracked: []string{}},
		PrimaryEntryPoints: []EntryPoint{}, MajorComponents: []Component{},
		ImportantDocumentation: []Document{}, RecentRelevantChanges: []string{}, ProjectSymbols: []ProjectSymbol{}, Uncertainties: []string{},
	}
	serialized, err := json.Marshal(answer)
	if err != nil {
		t.Fatal(err)
	}
	_, score, err := ParseAndScore(serialized, model.Basis{Branch: &branch, Staged: []string{}, Unstaged: []string{}, Untracked: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if score.Facts[0].Passed || score.Passed != score.Total-1 {
		t.Fatalf("unexpected score: %#v", score)
	}
}
