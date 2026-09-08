package evidence

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadReferenceValidatesContainmentAndExactBytes(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	content := []byte("external evidence fixture")
	for _, directory := range []string{root, outside} {
		if err := os.WriteFile(filepath.Join(directory, "record.json"), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(outside, "record.json"), filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("record.json", filepath.Join(root, "inside")); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path, digest string
		valid        bool
	}{
		{"record.json", Digest(content), true}, {"inside", Digest(content), true},
		{"record.json", Digest([]byte("wrong")), false}, {"missing", Digest(content), false},
		{filepath.Join(outside, "record.json"), Digest(content), false}, {"../record.json", Digest(content), false},
		{"nested/../record.json", Digest(content), false}, {"escape", Digest(content), false}, {".", Digest(content), false},
	} {
		t.Run(test.path+test.digest, func(t *testing.T) {
			data, err := ReadReference(root, FileReference{Path: test.path, Digest: test.digest})
			if (err == nil) != test.valid || (test.valid && string(data) != string(content)) {
				t.Fatalf("reference result: %s %v", data, err)
			}
		})
	}
}
