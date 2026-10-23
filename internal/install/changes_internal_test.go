package install

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// failAfterWriter fails at a chosen write, so the header failure and the
// line failure of WriteDiff are each reached.
type failAfterWriter struct {
	left int
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.left == 0 {
		return 0, io.ErrClosedPipe
	}
	w.left--
	return len(p), nil
}

func TestReadOptionalRefusesIrregularPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, _, err := readOptional(dir); !errors.Is(err, ErrPath) {
		t.Fatalf("a directory was read: %v", err)
	}
	file := filepath.Join(dir, "file")
	must(t, os.WriteFile(file, []byte("x"), 0o600))
	switch _, exists, err := readOptional(filepath.Join(file, "child")); {
	case notDirIsAbsent && (err != nil || exists):
		t.Fatalf("a path under a file, which this system reads as absent = %v, %v", exists, err)
	case !notDirIsAbsent && err == nil:
		t.Fatal("a path under a file was read")
	}
	data, exists, err := readOptional(file)
	if err != nil || !exists || string(data) != "x" {
		t.Fatalf("%q, %v, %v", data, exists, err)
	}
	if _, exists, err := readOptional(filepath.Join(dir, "absent")); err != nil || exists {
		t.Fatalf("absent = %v, %v", exists, err)
	}
}

func TestWriteDiffReportsWriterFailures(t *testing.T) {
	t.Parallel()
	change := Change{Path: "a", Before: []byte("old\n"), After: []byte("new\n"), Existed: true}
	if err := WriteDiff(&failAfterWriter{}, []Change{change}); err == nil {
		t.Fatal("a failed header was accepted")
	}
	if err := WriteDiff(&failAfterWriter{left: 1}, []Change{change}); err == nil {
		t.Fatal("a failed line was accepted")
	}
	var out bytes.Buffer
	must(t, WriteDiff(&out, []Change{change}))
	if !strings.Contains(out.String(), "-old\n") || !strings.Contains(out.String(), "+new\n") {
		t.Fatalf("diff = %q", out.String())
	}
}
