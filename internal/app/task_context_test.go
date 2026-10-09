package app_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/jeonghanlee/repocue/internal/cue"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

type taskFixture struct{ binary, root, cache, external string }

func buildTaskExecutable(t *testing.T, target string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "executable")
	command := exec.Command("go", "build", "-o", binary, target)
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v: %s", target, err, output)
	}
	return binary
}

func taskBytesDigest(data []byte) string {
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func newTaskFixture(t *testing.T, binary string) taskFixture {
	t.Helper()
	f := taskFixture{binary: binary, root: createGitRepository(t), cache: t.TempDir(), external: t.TempDir()}
	for _, name := range []string{"rules.md", "verification.md", "optional.md"} {
		value, err := os.ReadFile(filepath.Join("testdata", "task-context", name))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(f.root, "docs", name), string(value))
	}
	f.resetDeclaration(t)
	runGit(t, f.root, "add", "docs")
	f.cli(t, "init", "--cache-dir", f.cache, f.root)
	return f
}

func (f taskFixture) resetDeclaration(t *testing.T) {
	t.Helper()
	value, err := os.ReadFile(filepath.Join("testdata", "task-context", "context.json"))
	if err != nil {
		t.Fatal(err)
	}
	for marker, name := range map[string]string{"SOURCE_DIGEST": "rules.md", "CHECK_DIGEST": "verification.md", "OPTIONAL_DIGEST": "optional.md"} {
		source, err := os.ReadFile(filepath.Join(f.root, "docs", name))
		if err != nil {
			t.Fatal(err)
		}
		value = bytes.ReplaceAll(value, []byte(marker), []byte(taskBytesDigest(source)))
	}
	writeFile(t, filepath.Join(f.root, "docs", "context.json"), string(value))
}

func (f taskFixture) raw(args ...string) ([]byte, []byte, error) {
	command := exec.Command(f.binary, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func (f taskFixture) cli(t *testing.T, args ...string) []byte {
	t.Helper()
	stdout, stderr, err := f.raw(args...)
	if err != nil {
		t.Fatalf("CLI %v: %v: %s", args, err, stderr)
	}
	return stdout
}

func (f taskFixture) taskArgs(budget int, targets ...string) []string {
	args := []string{"cue", "--repository", f.root, "--cache-dir", f.cache, "--view", "task", "--context-file", "docs/context.json", "--task-kind", "explanation", "--max-tokens", strconv.Itoa(budget)}
	for _, target := range targets {
		args = append(args, "--target", target)
	}
	return args
}

func (f taskFixture) task(t *testing.T, budget int, targets ...string) cue.TaskEnvelope {
	t.Helper()
	value := f.cli(t, f.taskArgs(budget, targets...)...)
	result, err := cue.DecodeTask(value)
	if err != nil {
		t.Fatalf("task public contract: %v: %s", err, value)
	}
	return result
}

func (f taskFixture) fail(t *testing.T, args ...string) {
	t.Helper()
	stdout, stderr, err := f.raw(args...)
	if err == nil || len(stdout) != 0 || len(stderr) == 0 {
		t.Fatalf("failure must have no successful stdout: %v %s %s", err, stdout, stderr)
	}
}

func TestTaskCueShippedSelectionAndOriginalBytes(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	f := newTaskFixture(t, binary)
	result := f.task(t, 5000, "cmd/example/new.go")
	if !reflect.DeepEqual(result.Content.RequiredIDs, []string{"global-rule", "scoped-check"}) || len(result.Content.Sources) != 3 {
		t.Fatal("required selection or source deduplication failed")
	}
	source, err := os.ReadFile(filepath.Join(f.root, "docs", "rules.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, delivered := range result.Content.Sources {
		if delivered.Path == "docs/rules.md" {
			want := bytes.SplitAfter(source, []byte("\n"))
			selected := bytes.Join(want[1:3], nil)
			if delivered.Text != string(selected) || delivered.TextDigest != taskBytesDigest(selected) || delivered.ContentDigest != taskBytesDigest(source) {
				t.Fatal("source bytes or digests differ")
			}
		}
	}
	onlyGlobal := f.task(t, 5000, "internal/application/new.go")
	if !reflect.DeepEqual(onlyGlobal.Content.RequiredIDs, []string{"global-rule"}) {
		t.Fatal("unrelated required context selected")
	}
	directory := f.task(t, 5000, "cmd")
	if !reflect.DeepEqual(directory.Content.RequiredIDs, result.Content.RequiredIDs) {
		t.Fatal("directory intersection lost context")
	}
	first := f.cli(t, f.taskArgs(5000, "cmd/example/new.go")...)
	second := f.cli(t, f.taskArgs(5000, "cmd/example/new.go", "cmd/example/new.go")...)
	if !bytes.Equal(first, second) {
		t.Fatal("normalized repeated request is not deterministic")
	}
	for _, args := range [][]string{
		{"cue", "--repository", f.root, "--cache-dir", f.cache, "--view", "task"},
		{"cue", "--repository", f.root, "--cache-dir", f.cache, "--target", ""},
		append(f.taskArgs(5000, "cmd"), "--path", ""),
		append(f.taskArgs(5000, "cmd"), "--since", ""),
		f.taskArgs(5000, "../escape"),
	} {
		f.fail(t, args...)
	}
}

func TestTaskCueShippedBudgetAndPinnedRefresh(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	f := newTaskFixture(t, binary)
	writeFile(t, filepath.Join(f.root, "docs", "optional.md"), strings.Repeat("Optional large background.\n", 10000))
	f.resetDeclaration(t)
	f.cli(t, "refresh", "--repository", f.root, "--cache-dir", f.cache)
	result := f.task(t, 5000, "cmd")
	if result.Content.OmittedCount != 1 || result.Content.Omitted[0].ID != "optional-background" || len(result.Content.Items) != 2 {
		t.Fatal("whole optional omission failed")
	}
	budget := result.Budget.EstimatedTokens
	for range 4 {
		result = f.task(t, budget, "cmd")
		if result.Budget.EstimatedTokens == budget {
			break
		}
		budget = result.Budget.EstimatedTokens
	}
	f.fail(t, f.taskArgs(budget-1, "cmd")...)
	original, err := os.ReadFile(filepath.Join(f.root, "docs", "rules.md"))
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(original, []byte("Keep"), []byte("Preserve"), 1)
	writeFile(t, filepath.Join(f.root, "docs", "rules.md"), string(changed))
	f.fail(t, f.taskArgs(5000, "cmd")...)
	f.cli(t, "refresh", "--repository", f.root, "--cache-dir", f.cache)
	f.fail(t, f.taskArgs(5000, "cmd")...)
	f.resetDeclaration(t)
	f.cli(t, "refresh", "--repository", f.root, "--cache-dir", f.cache)
	if f.task(t, 5000, "cmd").Freshness != "dirty-but-indexed" {
		t.Fatal("matching dirty snapshot rejected")
	}
	if err := os.Remove(filepath.Join(f.root, "docs", "rules.md")); err != nil {
		t.Fatal(err)
	}
	f.cli(t, "refresh", "--repository", f.root, "--cache-dir", f.cache)
	f.fail(t, f.taskArgs(5000, "cmd")...)
}

func TestTaskCueShippedSourceFailures(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	for _, mode := range []string{"range", "utf8", "symlink", "declaration-symlink", "intermediate-symlink", "absolute", "untracked", "gitlink", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			f := newTaskFixture(t, binary)
			name := filepath.Join(f.root, "docs", "context.json")
			raw, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			var declaration taskcontext.Declaration
			if err := json.Unmarshal(raw, &declaration); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "range":
				declaration.Items[0].Source.Lines = &taskcontext.Lines{Start: 1, End: 999}
			case "utf8":
				writeFile(t, filepath.Join(f.root, "docs", "rules.md"), string([]byte{0xff, '\n'}))
				declaration.Items[0].Source.Lines = nil
				declaration.Items[0].Source.ContentDigest = taskBytesDigest([]byte{0xff, '\n'})
			case "symlink":
				target := filepath.Join(f.external, "rules.md")
				writeFile(t, target, "external original\n")
				link := filepath.Join(f.root, "docs", "linked.md")
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
				runGit(t, f.root, "add", "docs/linked.md")
				declaration.Items[0].Source = taskcontext.Reference{Path: "docs/linked.md", ContentDigest: taskBytesDigest([]byte(target))}
			case "untracked":
				writeFile(t, filepath.Join(f.root, "docs", "untracked.md"), "untracked\n")
				declaration.Items[0].Source = taskcontext.Reference{Path: "docs/untracked.md", ContentDigest: taskBytesDigest([]byte("untracked\n"))}
			case "gitlink":
				head := exec.Command("git", "-C", f.root, "rev-parse", "HEAD")
				value, err := head.Output()
				if err != nil {
					t.Fatal(err)
				}
				runGit(t, f.root, "update-index", "--add", "--cacheinfo", "160000,"+strings.TrimSpace(string(value))+",module")
				declaration.Items[0].Source.Path = "module"
			case "duplicate":
				declaration.Items = append(declaration.Items, declaration.Items[0])
			case "absolute":
				declaration.Items[0].Source.Path = filepath.Join(f.external, "outside.md")
			case "declaration-symlink":
				external := filepath.Join(f.external, "context.json")
				writeFile(t, external, string(raw))
				if err := os.Rename(name, name+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(external, name); err != nil {
					t.Fatal(err)
				}
			case "intermediate-symlink":
				if err := os.Rename(filepath.Join(f.root, "docs"), filepath.Join(f.external, "original-docs")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(f.external, "original-docs"), filepath.Join(f.root, "docs")); err != nil {
					t.Fatal(err)
				}
			}
			encoded, err := json.Marshal(declaration)
			if err != nil {
				t.Fatal(err)
			}
			if mode != "declaration-symlink" {
				writeFile(t, name, string(encoded))
			}
			f.cli(t, "refresh", "--repository", f.root, "--cache-dir", f.cache)
			f.fail(t, f.taskArgs(5000, "cmd")...)
		})
	}
}

func TestTaskCueShippedLineEndingsAndLargeUTF8Boundary(t *testing.T) {
	binary := buildTaskExecutable(t, "./cmd/repocue")
	f := newTaskFixture(t, binary)
	source := strings.Repeat("a", 65535) + "\u03b1\r\nlast line"
	writeFile(t, filepath.Join(f.root, "docs", "rules.md"), source)
	f.resetDeclaration(t)
	name := filepath.Join(f.root, "docs", "context.json")
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var declaration taskcontext.Declaration
	if err := json.Unmarshal(raw, &declaration); err != nil {
		t.Fatal(err)
	}
	for i := range declaration.Items {
		if declaration.Items[i].Source.Path == "docs/rules.md" {
			declaration.Items[i].Source.Lines = &taskcontext.Lines{Start: 1, End: 2}
		}
	}
	encoded, _ := json.Marshal(declaration)
	writeFile(t, name, string(encoded))
	f.cli(t, "refresh", "--repository", f.root, "--cache-dir", f.cache)
	result := f.task(t, 50000, "cmd")
	for _, delivered := range result.Content.Sources {
		if delivered.Path == "docs/rules.md" && (delivered.Text != source || delivered.TextDigest != taskBytesDigest([]byte(source))) {
			t.Fatal("UTF-8 boundary, CRLF or final unterminated line changed")
		}
	}
}
