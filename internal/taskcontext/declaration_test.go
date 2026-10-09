package taskcontext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTaskDeclarationRejectsAmbiguity(t *testing.T) {
	d := Declaration{SchemaVersion: DeclarationVersion, TaskKinds: []string{"edit"},
		Items: []Item{{ID: "rule", Kind: "instruction", Required: true,
			Source: Reference{Path: "rules.md", ContentDigest: Digest([]byte("rule"))}, Checks: []Reference{}}}}
	serialized, _ := json.Marshal(d)
	if _, err := DecodeDeclaration(serialized); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		strings.Replace(string(serialized), "\"id\":\"rule\"", "\"id\":\"rule\",\"id\":\"other\"", 1),
		strings.Replace(string(serialized), "\"items\":", "\"unknown\":0,\"items\":", 1),
		strings.Replace(string(serialized), "\"task_kinds\":null", "\"task_kinds\":[]", 1),
		strings.Replace(string(serialized), "\"path_prefixes\":null", "\"path_prefixes\":[\"../escape\"]", 1),
		strings.Replace(string(serialized), "\"lines\":null", "\"lines\":{\"start\":2,\"end\":1}", 1),
		strings.Replace(string(serialized), "\"task_kinds\":null", "\"task_kinds\":[\"unknown\"]", 1),
		string(serialized) + " {}",
	} {
		if _, err := DecodeDeclaration([]byte(value)); err == nil {
			t.Fatalf("invalid declaration accepted: %s", value)
		}
	}
	d.Items = append(d.Items, d.Items[0])
	serialized, _ = json.Marshal(d)
	if _, err := DecodeDeclaration(serialized); err == nil {
		t.Fatal("duplicate item IDs accepted")
	}
}

func TestTaskSelectorComponentAndDirectoryScope(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := Declaration{TaskKinds: []string{"edit"}, Items: []Item{
		{ID: "global"}, {ID: "scope", TaskKinds: []string{"edit"}, PathPrefixes: []string{"internal/app"}},
		{ID: "other-task", TaskKinds: []string{"inspect"}}, {ID: "sibling", PathPrefixes: []string{"internal/application"}},
	}}
	for _, test := range []struct {
		targets []string
		want    []string
	}{
		{[]string{"internal/app/new.go"}, []string{"global", "scope"}},
		{[]string{"internal"}, []string{"global", "scope", "sibling"}},
		{[]string{"internal/application/new.go"}, []string{"global", "sibling"}},
		{[]string{"internal/app/new.go", "internal/application/new.go"}, []string{"global", "scope", "sibling"}},
		{[]string{"."}, []string{"global", "scope", "sibling"}},
	} {
		items, _, err := Select(d, root, Request{TaskKind: "edit", Targets: test.targets})
		if err != nil {
			t.Fatal(err)
		}
		ids := []string{}
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		if !reflect.DeepEqual(ids, test.want) {
			t.Fatalf("%v: %v want %v", test.targets, ids, test.want)
		}
	}
	for _, request := range []Request{{}, {TaskKind: "unknown", Targets: []string{"."}}, {TaskKind: "edit", Targets: []string{"a/../b"}}} {
		if _, _, err := Select(d, root, request); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
}
