package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAtomicFailuresLeaveNoTemporaryFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	must(t, os.WriteFile(file, []byte("x"), 0o600))
	if err := WriteAtomic(filepath.Join(file, "child"), []byte("x")); err == nil {
		t.Fatal("a path under a file was written")
	}
	occupied := filepath.Join(dir, "occupied")
	must(t, os.MkdirAll(filepath.Join(occupied, "child"), 0o700))
	if err := WriteAtomic(occupied, []byte("x")); err == nil {
		t.Fatal("a directory was replaced by an artifact")
	}
	entries, err := os.ReadDir(dir)
	must(t, err)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".golaw-") {
			t.Fatalf("temporary file survived: %s", entry.Name())
		}
	}
	if absentErr := removeTemporary(filepath.Join(dir, "absent")); absentErr != nil {
		t.Fatal(absentErr)
	}
	if dirErr := removeTemporary(occupied); dirErr == nil {
		t.Fatal("a directory was removed as a temporary file")
	}
	closed, err := os.CreateTemp(dir, "closed-*")
	must(t, err)
	must(t, closed.Close())
	if err := writeTemporary(closed, []byte("x")); err == nil {
		t.Fatal("a closed file was written")
	}
}
