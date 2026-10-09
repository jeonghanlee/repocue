package cue_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

func TestTaskCueCodecPreservesOriginalJSONEscaping(t *testing.T) {
	original, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	digest := taskcontext.Digest(original)
	reference := taskcontext.Reference{Path: "README.md", ContentDigest: digest}
	value := cue.TaskEnvelope{SchemaVersion: cue.TaskSchemaVersion, Kind: "task",
		Repository: cue.Repository{ID: "repo-fixture", Name: "fixture"}, Epoch: "epoch-000001", Snapshot: "snapshot-000001",
		Basis:     cue.Basis{StatusDigest: digest, RepositoryDigest: digest, ObservedAt: "2026-10-08T00:00:00Z"},
		Freshness: "current", Budget: cue.Budget{MaxTokens: 5000, EstimatedTokens: 1000},
		Content: cue.TaskContent{Declaration: cue.DeclarationIdentity{Path: "docs/context.json", ContentDigest: digest},
			Request: taskcontext.Request{TaskKind: "discovery", Targets: []string{"."}}, RequiredIDs: []string{}, Items: []cue.TaskItem{},
			Sources: []taskcontext.Source{{ID: taskcontext.ReferenceID(reference), Path: "README.md", ContentDigest: digest, Text: string(original), TextDigest: digest}},
			Omitted: []cue.Omission{}}, Warnings: []cue.Warning{}, More: []cue.More{}}
	serialized, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := cue.DecodeTask(serialized)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(decoded.Content.Sources[0].Text), original) {
		t.Fatal("JSON escaping changed original text")
	}
	duplicate := strings.Replace(string(serialized), "\"kind\":\"task\"", "\"kind\":\"task\",\"kind\":\"task\"", 1)
	if _, err := cue.DecodeTask([]byte(duplicate)); err == nil {
		t.Fatal("duplicate public envelope member accepted")
	}
	value.SchemaVersion = "repocue/1"
	serialized, _ = json.Marshal(value)
	if _, err := cue.DecodeTask(serialized); err == nil {
		t.Fatal("legacy envelope accepted as task context")
	}
}
