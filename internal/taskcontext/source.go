package taskcontext

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/jeonghanlee/repocue/internal/model"
)

const sourceBufferBytes = 64 * 1024

type Source struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	ContentDigest string `json:"content_digest"`
	Lines         *Lines `json:"lines"`
	Text          string `json:"text"`
	TextDigest    string `json:"text_digest"`
	Overflow      bool   `json:"-"`
}

type Reader struct {
	root  *os.Root
	files map[string]model.File
}

func NewReader(state model.CurrentState) (*Reader, error) {
	root, err := os.OpenRoot(state.Repository.Root)
	if err != nil {
		return nil, err
	}
	files := make(map[string]model.File, len(state.Files))
	for _, file := range state.Files {
		files[file.Path] = file
	}
	return &Reader{root: root, files: files}, nil
}

func (reader *Reader) Close() error { return reader.root.Close() }

func (reader *Reader) Declaration(ctx context.Context, name string) (Declaration, string, error) {
	file, found := reader.files[name]
	if !found {
		return Declaration{}, "", errors.New("context declaration is not tracked")
	}
	source, err := reader.Read(ctx, Reference{Path: name, ContentDigest: file.ContentDigest}, MaxDeclarationBytes)
	if err != nil {
		return Declaration{}, "", err
	}
	if source.Overflow {
		return Declaration{}, "", errors.New("context declaration exceeds the 1 MiB limit")
	}
	declaration, err := DecodeDeclaration([]byte(source.Text))
	return declaration, source.ContentDigest, err
}

// Read streams the complete file identity and UTF-8 validation while retaining
// at most maxBytes of the selected range. Overflow never returns partial text.
func (reader *Reader) Read(ctx context.Context, reference Reference, maxBytes int) (Source, error) {
	source := Source{ID: ReferenceID(reference), Path: reference.Path,
		ContentDigest: reference.ContentDigest, Lines: reference.Lines}
	if err := ValidateReference(reference); err != nil {
		return source, err
	}
	indexed, found := reader.files[reference.Path]
	if !found || !indexed.Exists ||
		(indexed.IndexMode != "100644" && indexed.IndexMode != "100755") ||
		(indexed.WorkingTreeMode != "100644" && indexed.WorkingTreeMode != "100755") {
		return source, fmt.Errorf("source must be a tracked regular file: %s", reference.Path)
	}
	if indexed.ContentDigest != reference.ContentDigest {
		return source, fmt.Errorf("source digest differs from snapshot: %s", reference.Path)
	}
	before, err := reader.noLinks(reference.Path)
	if err != nil {
		return source, err
	}
	file, err := reader.root.Open(reference.Path)
	if err != nil {
		return source, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return source, errors.New("source changed while opening")
	}
	contentHash, textHash := sha256.New(), sha256.New()
	var text bytes.Buffer
	buffer := bufio.NewReaderSize(file, sourceBufferBytes)
	line, lastLine := 1, 0
	var pending []byte
	for {
		if err := ctx.Err(); err != nil {
			return source, err
		}
		chunk, readErr := buffer.ReadSlice('\n')
		if len(chunk) > 0 {
			lastLine = line
			contentHash.Write(chunk)
			pending, err = validateChunk(pending, chunk)
			if err != nil {
				return source, fmt.Errorf("%s: %w", reference.Path, err)
			}
			if reference.Lines == nil || (line >= reference.Lines.Start && line <= reference.Lines.End) {
				textHash.Write(chunk)
				if !source.Overflow {
					if len(chunk) > maxBytes-text.Len() {
						source.Overflow = true
						text.Reset()
					} else {
						text.Write(chunk)
					}
				}
			}
			if chunk[len(chunk)-1] == '\n' {
				line++
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil && readErr != bufio.ErrBufferFull {
			return source, readErr
		}
	}
	if len(pending) > 0 {
		return source, errors.New("source ends in incomplete UTF-8")
	}
	if reference.Lines != nil && reference.Lines.End > lastLine {
		return source, errors.New("source range exceeds the actual file")
	}
	actualDigest := "sha256:" + hex.EncodeToString(contentHash.Sum(nil))
	after, err := file.Stat()
	if err != nil {
		return source, err
	}
	pathAfter, err := reader.noLinks(reference.Path)
	if err != nil {
		return source, err
	}
	if !os.SameFile(before, after) || !os.SameFile(after, pathAfter) ||
		before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) ||
		before.Mode() != after.Mode() || after.Size() != indexed.SizeBytes ||
		actualDigest != indexed.ContentDigest {
		return source, fmt.Errorf("source no longer matches the pinned snapshot: %s", reference.Path)
	}
	source.TextDigest = "sha256:" + hex.EncodeToString(textHash.Sum(nil))
	if !source.Overflow {
		source.Text = text.String()
	}
	return source, nil
}

func (reader *Reader) noLinks(name string) (os.FileInfo, error) {
	var info os.FileInfo
	for index, component := range strings.Split(name, "/") {
		prefix := strings.Join(strings.Split(name, "/")[:index+1], "/")
		var err error
		info, err = os.Lstat(filepath.Join(reader.root.Name(), filepath.FromSlash(prefix)))
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("source has a linked component: %s", component)
		}
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("source is not a regular file")
	}
	return info, nil
}

func validateChunk(pending, chunk []byte) ([]byte, error) {
	value := append(pending, chunk...)
	end := len(value)
	if end > 0 {
		start := end - 1
		for start > 0 && !utf8.RuneStart(value[start]) {
			start--
		}
		if !utf8.FullRune(value[start:]) {
			end = start
		}
	}
	if !utf8.Valid(value[:end]) {
		return nil, errors.New("source is not valid UTF-8")
	}
	return append([]byte(nil), value[end:]...), nil
}
