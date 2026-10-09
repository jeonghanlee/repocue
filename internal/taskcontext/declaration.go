// Package taskcontext selects explicitly declared repository context and reads
// pinned original sources without storing text in the repository cache.
package taskcontext

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/jeonghanlee/repocue/docs/schema"
)

const (
	DeclarationVersion  = "repocue/task-context-1"
	MaxDeclarationBytes = 1024 * 1024
)

type Lines struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type Reference struct {
	Path          string `json:"path"`
	ContentDigest string `json:"content_digest"`
	Lines         *Lines `json:"lines"`
}

type Item struct {
	ID           string      `json:"id"`
	Kind         string      `json:"kind"`
	Required     bool        `json:"required"`
	TaskKinds    []string    `json:"task_kinds"`
	PathPrefixes []string    `json:"path_prefixes"`
	Source       Reference   `json:"source"`
	Checks       []Reference `json:"checks"`
}

type Declaration struct {
	SchemaVersion string   `json:"schema_version"`
	TaskKinds     []string `json:"task_kinds"`
	Items         []Item   `json:"items"`
}

type Request struct {
	TaskKind string   `json:"task_kind"`
	Targets  []string `json:"targets"`
}

// DecodeStrict rejects duplicate members before schema validation can discard
// them, and rejects malformed UTF-8, unknown fields and trailing JSON values.
func DecodeStrict(serialized []byte, contract string, target any) error {
	if !utf8.Valid(serialized) {
		return errors.New("JSON is not valid UTF-8")
	}
	tokens := json.NewDecoder(bytes.NewReader(serialized))
	tokens.UseNumber()
	if err := uniqueMembers(tokens); err != nil {
		return err
	}
	if _, err := tokens.Token(); err != io.EOF {
		return errors.New("JSON must contain exactly one value")
	}
	if contract != "" {
		if err := schema.Validate(contract, serialized); err != nil {
			return err
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(serialized))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func uniqueMembers(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := token.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate or invalid JSON member %q", token)
			}
			seen[name] = true
			if err := uniqueMembers(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := uniqueMembers(decoder); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}

func DecodeDeclaration(serialized []byte) (Declaration, error) {
	var declaration Declaration
	if err := DecodeStrict(serialized, "task-context-v1.schema.json", &declaration); err != nil {
		return declaration, err
	}
	kinds := map[string]bool{}
	for _, kind := range declaration.TaskKinds {
		if kind == "" || strings.TrimSpace(kind) != kind {
			return declaration, errors.New("task kind must be a nonempty canonical name")
		}
		kinds[kind] = true
	}
	ids := map[string]bool{}
	for _, item := range declaration.Items {
		if ids[item.ID] || strings.TrimSpace(item.ID) != item.ID {
			return declaration, fmt.Errorf("duplicate or invalid item ID %q", item.ID)
		}
		ids[item.ID] = true
		for _, kind := range item.TaskKinds {
			if !kinds[kind] {
				return declaration, fmt.Errorf("item %s uses unknown task kind %s", item.ID, kind)
			}
		}
		for _, prefix := range item.PathPrefixes {
			if err := ValidatePath(prefix, true); err != nil {
				return declaration, err
			}
		}
		for _, reference := range append([]Reference{item.Source}, item.Checks...) {
			if err := ValidateReference(reference); err != nil {
				return declaration, err
			}
		}
	}
	return declaration, nil
}

func ValidatePath(name string, allowRoot bool) error {
	if name == "" || (!allowRoot && name == ".") || path.Clean(name) != name ||
		!filepath.IsLocal(name) || strings.ContainsAny(name, "\\:\x00") {
		return fmt.Errorf("path must be canonical and repository-relative: %q", name)
	}
	for _, char := range name {
		if char < 32 || char == 127 {
			return fmt.Errorf("path contains a control character: %q", name)
		}
	}
	return nil
}

func ValidateReference(reference Reference) error {
	if err := ValidatePath(reference.Path, false); err != nil {
		return err
	}
	if len(reference.ContentDigest) != len("sha256:")+sha256.Size*2 ||
		!strings.HasPrefix(reference.ContentDigest, "sha256:") {
		return errors.New("reference requires a SHA-256 full-file digest")
	}
	hexPart := strings.TrimPrefix(reference.ContentDigest, "sha256:")
	if _, err := hex.DecodeString(hexPart); err != nil || strings.ToLower(hexPart) != hexPart {
		return errors.New("reference digest is not canonical hexadecimal")
	}
	if reference.Lines != nil && (reference.Lines.Start < 1 || reference.Lines.End < reference.Lines.Start) {
		return errors.New("source line range must be inclusive and 1-based")
	}
	return nil
}

func Digest(data []byte) string {
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func ReferenceID(reference Reference) string {
	serialized, _ := json.Marshal(reference)
	return Digest(serialized)
}
