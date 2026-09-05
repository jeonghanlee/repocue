package benchmark

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"

	"github.com/jeonghanlee/repocue/internal/model"
)

const (
	Version             = "repository-state-v2"
	AnswerSchemaVersion = "repocue/benchmark-answer-2"
)

type GitState struct {
	Branch         *string  `json:"branch"`
	Head           *string  `json:"head"`
	Dirty          bool     `json:"dirty"`
	TrackedChanges []string `json:"tracked_changes"`
	Untracked      []string `json:"untracked"`
}

type EntryPoint struct {
	Path           string `json:"path"`
	Responsibility string `json:"responsibility"`
}

type Component struct {
	Name             string   `json:"name"`
	Responsibilities []string `json:"responsibilities"`
	Paths            []string `json:"paths"`
}

type Document struct {
	Path    string `json:"path"`
	Purpose string `json:"purpose"`
}

type ProjectSymbol struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Owner     string `json:"owner"`
	Relevance string `json:"relevance"`
}

type Answer struct {
	SchemaVersion          string          `json:"schema_version"`
	ProjectPurpose         string          `json:"project_purpose"`
	Git                    GitState        `json:"git"`
	PrimaryEntryPoints     []EntryPoint    `json:"primary_entry_points"`
	MajorComponents        []Component     `json:"major_components"`
	ImportantDocumentation []Document      `json:"important_documentation"`
	RecentRelevantChanges  []string        `json:"recent_relevant_changes"`
	ProjectSymbols         []ProjectSymbol `json:"project_symbols"`
	Uncertainties          []string        `json:"uncertainties"`
}

type FactScore struct {
	Fact     string `json:"fact"`
	Passed   bool   `json:"passed"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
}

type DeterministicScore struct {
	Status string      `json:"status"`
	Passed int         `json:"passed"`
	Total  int         `json:"total"`
	Ratio  float64     `json:"ratio"`
	Facts  []FactScore `json:"facts"`
}

func ParseAndScore(serialized json.RawMessage, basis model.Basis) (Answer, DeterministicScore, error) {
	decoder := json.NewDecoder(bytes.NewReader(serialized))
	decoder.DisallowUnknownFields()
	var answer Answer
	if err := decoder.Decode(&answer); err != nil {
		return Answer{}, DeterministicScore{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Answer{}, DeterministicScore{}, errors.New("benchmark answer contains more than one JSON value")
	}
	if answer.SchemaVersion != AnswerSchemaVersion {
		return Answer{}, DeterministicScore{}, errors.New("unsupported benchmark answer schema version")
	}
	if err := validateAnswerShape(serialized); err != nil {
		return Answer{}, DeterministicScore{}, err
	}

	expectedTracked := append([]string{}, basis.Staged...)
	expectedTracked = append(expectedTracked, basis.Unstaged...)
	expectedTracked = sortedUnique(expectedTracked)
	actualTracked := sortedUnique(answer.Git.TrackedChanges)
	expectedUntracked := sortedUnique(basis.Untracked)
	actualUntracked := sortedUnique(answer.Git.Untracked)
	facts := []FactScore{
		{Fact: "branch", Passed: equalPointers(answer.Git.Branch, basis.Branch), Expected: basis.Branch, Actual: answer.Git.Branch},
		{Fact: "head", Passed: equalPointers(answer.Git.Head, basis.Head), Expected: basis.Head, Actual: answer.Git.Head},
		{Fact: "dirty", Passed: answer.Git.Dirty == basis.Dirty, Expected: basis.Dirty, Actual: answer.Git.Dirty},
		{Fact: "tracked_changes", Passed: slices.Equal(actualTracked, expectedTracked), Expected: expectedTracked, Actual: actualTracked},
		{Fact: "untracked", Passed: slices.Equal(actualUntracked, expectedUntracked), Expected: expectedUntracked, Actual: actualUntracked},
	}
	passed := 0
	for _, fact := range facts {
		if fact.Passed {
			passed++
		}
	}
	return answer, DeterministicScore{
		Status: "observed", Passed: passed, Total: len(facts),
		Ratio: float64(passed) / float64(len(facts)), Facts: facts,
	}, nil
}

func validateAnswerShape(serialized json.RawMessage) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(serialized, &root); err != nil {
		return err
	}
	if err := requireFields(root, false,
		"schema_version", "project_purpose", "git", "primary_entry_points", "major_components",
		"important_documentation", "recent_relevant_changes", "project_symbols", "uncertainties"); err != nil {
		return fmt.Errorf("benchmark answer: %w", err)
	}
	git, err := objectValue(root["git"], "git")
	if err != nil {
		return err
	}
	if err := requireFields(git, true, "branch", "head"); err != nil {
		return fmt.Errorf("benchmark answer git: %w", err)
	}
	if err := requireFields(git, false, "dirty", "tracked_changes", "untracked"); err != nil {
		return fmt.Errorf("benchmark answer git: %w", err)
	}
	for _, name := range []string{"tracked_changes", "untracked"} {
		if err := stringArrayValue(git[name], "git."+name); err != nil {
			return err
		}
	}
	for _, name := range []string{"recent_relevant_changes", "uncertainties"} {
		if err := stringArrayValue(root[name], name); err != nil {
			return err
		}
	}
	if err := objectArrayValue(root["primary_entry_points"], "primary_entry_points", []string{"path", "responsibility"}, nil); err != nil {
		return err
	}
	if err := objectArrayValue(root["major_components"], "major_components", []string{"name", "responsibilities", "paths"}, []string{"responsibilities", "paths"}); err != nil {
		return err
	}
	if err := objectArrayValue(root["important_documentation"], "important_documentation", []string{"path", "purpose"}, nil); err != nil {
		return err
	}
	return objectArrayValue(root["project_symbols"], "project_symbols", []string{"name", "signature", "owner", "relevance"}, nil)
}

func requireFields(object map[string]json.RawMessage, allowNull bool, names ...string) error {
	for _, name := range names {
		value, found := object[name]
		if !found {
			return fmt.Errorf("missing required field %q", name)
		}
		if !allowNull && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("required field %q must not be null", name)
		}
	}
	return nil
}

func objectValue(serialized json.RawMessage, name string) (map[string]json.RawMessage, error) {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(serialized, &value); err != nil || value == nil {
		return nil, fmt.Errorf("benchmark answer field %q must be an object", name)
	}
	return value, nil
}

func stringArrayValue(serialized json.RawMessage, name string) error {
	var values []json.RawMessage
	if err := json.Unmarshal(serialized, &values); err != nil || values == nil {
		return fmt.Errorf("benchmark answer field %q must be an array", name)
	}
	for _, value := range values {
		var decoded string
		if err := json.Unmarshal(value, &decoded); err != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("benchmark answer field %q must contain strings", name)
		}
	}
	return nil
}

func objectArrayValue(serialized json.RawMessage, name string, required, stringArrays []string) error {
	var values []json.RawMessage
	if err := json.Unmarshal(serialized, &values); err != nil || values == nil {
		return fmt.Errorf("benchmark answer field %q must be an array", name)
	}
	for index, value := range values {
		item, err := objectValue(value, fmt.Sprintf("%s[%d]", name, index))
		if err != nil {
			return err
		}
		if err := requireFields(item, false, required...); err != nil {
			return fmt.Errorf("benchmark answer %s[%d]: %w", name, index, err)
		}
		for _, field := range stringArrays {
			if err := stringArrayValue(item[field], fmt.Sprintf("%s[%d].%s", name, index, field)); err != nil {
				return err
			}
		}
	}
	return nil
}

func equalPointers(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func sortedUnique(values []string) []string {
	result := append([]string{}, values...)
	sort.Strings(result)
	if len(result) == 0 {
		return []string{}
	}
	write := 1
	for read := 1; read < len(result); read++ {
		if result[read] == result[write-1] {
			continue
		}
		result[write] = result[read]
		write++
	}
	return result[:write]
}
