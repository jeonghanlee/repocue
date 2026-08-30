package benchmark

import (
	"bytes"
	"encoding/json"
	"errors"
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
