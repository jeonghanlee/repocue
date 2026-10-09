package taskcontext_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeonghanlee/repocue/internal/model"
	"github.com/jeonghanlee/repocue/internal/repository"
	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

func TestTaskSourceStreamValidatesWholeFileAfterOverflow(t *testing.T) {
	for _, test := range []struct {
		name, content   string
		lines           *taskcontext.Lines
		budget          int
		valid, overflow bool
	}{
		{"empty", "", nil, 0, true, false},
		{"empty range", "", &taskcontext.Lines{Start: 1, End: 1}, 100, false, false},
		{"exact CRLF", "original\r\nlast", nil, 14, true, false},
		{"overflow clears text", strings.Repeat("original\n", 10000), nil, 5, true, true},
		{"invalid outside range", "original\n" + string([]byte{0xff}), &taskcontext.Lines{Start: 1, End: 1}, 100, false, false},
		{"invalid after overflow", strings.Repeat("original\n", 10000) + string([]byte{0xff}), nil, 5, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			for _, args := range [][]string{{"init", "-b", "main"}, {"add", "original.txt"}} {
				if args[0] == "add" {
					if err := os.WriteFile(filepath.Join(root, "original.txt"), []byte(test.content), 0644); err != nil {
						t.Fatal(err)
					}
				}
				command := exec.Command("git", append([]string{"-C", root}, args...)...)
				if value, err := command.CombinedOutput(); err != nil {
					t.Fatalf("real tracked source: %v %s", err, value)
				}
			}
			ctx := context.Background()
			repo, err := repository.Open(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			scan, err := repo.FullScan(ctx)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := taskcontext.NewReader(model.CurrentState{Repository: model.Repository{Root: repo.Root}, Files: scan.Files})
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			result, err := reader.Read(ctx, taskcontext.Reference{Path: "original.txt", ContentDigest: scan.Files[0].ContentDigest, Lines: test.lines}, test.budget)
			if (err == nil) != test.valid {
				t.Fatalf("stream validation: %v", err)
			}
			if test.valid && (result.Overflow != test.overflow || (result.Overflow && result.Text != "")) {
				t.Fatalf("bounded original text: %+v", result)
			}
			if test.valid && !result.Overflow && result.Text != test.content {
				t.Fatal("original bytes changed")
			}
		})
	}
}
