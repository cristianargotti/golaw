package runner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestTreePayloadRequiresExactRecords(t *testing.T) {
	t.Parallel()
	object := strings.Repeat("a", 40)
	header := object + " blob 4\n"
	for _, tc := range []struct {
		name, stream string
		bad          bool
	}{
		{"near-exact", header + "a\x00b\n\n", false},
		{"red-identity", strings.Repeat("b", 40) + " blob 4\na\x00b\n\n", true},
		{"red-size", object + " blob 3\na\x00b\n", true},
		{"red-truncated", header + "a", true},
		{"red-separator", header + "a\x00b\nx", true},
		{"red-missing-separator", header + "a\x00b\n", true},
		{"red-extra-record", header + "a\x00b\n\n" + header, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := payloadRoot(t)
			entries := []treeEntry{{path: "value.txt", mode: "100644", object: object, size: 4}}
			err := writeTreeStream(t.Context(), root, bufio.NewReader(strings.NewReader(tc.stream)), entries, copyTreeBlob)
			if (err != nil) != tc.bad {
				t.Fatalf("payload error = %v", err)
			}
			if !tc.bad {
				data, readErr := root.ReadFile("value.txt")
				if readErr != nil || string(data) != "a\x00b\n" {
					t.Fatalf("payload bytes = %q, %v", data, readErr)
				}
			}
		})
	}
}

func payloadRoot(t *testing.T) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	return root
}

type shortTreeWriter struct{}

func (shortTreeWriter) Write(data []byte) (int, error) { return len(data) - 1, nil }

func TestTreeCopyPreservesFailures(t *testing.T) {
	t.Parallel()
	if err := copyTreeBlob(t.Context(), shortTreeWriter{}, strings.NewReader("value"), 5); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write = %v", err)
	}
	if err := copyTreeBlob(t.Context(), io.Discard, brokenRefReaderInternal{}, 5); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("read failure = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, size := range []int64{0, 5} {
		if err := copyTreeBlob(ctx, io.Discard, strings.NewReader("value"), size); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled copy = %v", err)
		}
	}
}

type brokenRefReaderInternal struct{}

func (brokenRefReaderInternal) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestMaterializePreservesPayloadFailureAndCleans(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{"value.txt": "value"})
	parent := t.TempDir()
	opened := false
	copyBlob := func(_ context.Context, dst io.Writer, _ io.Reader, _ int64) error {
		file, ok := dst.(*os.File)
		if !ok {
			t.Fatal("payload destination was not opened")
		}
		_, err := file.Stat()
		opened = err == nil
		return io.ErrClosedPipe
	}
	tree, err := materialize(t.Context(), MaterializeInput{
		Dir: repo.Root, Env: gittest.Isolated(), Parent: parent, Bounds: DefaultBounds(),
	}, copyBlob)
	if !opened || tree != nil || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("failed payload = %v, %v, opened=%t", tree, err, opened)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("resources remain: %v, %v", entries, err)
	}
}

func TestTreePayloadPreservesFilesAndGitlinks(t *testing.T) {
	t.Parallel()
	root := payloadRoot(t)
	if err := root.WriteFile("value", []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := treeEntry{path: "value", mode: "100755", size: 3}
	if err := writeTreeBlob(t.Context(), root, strings.NewReader("new"), entry, copyTreeBlob); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing destination = %v", err)
	}
	entry.path = "executable"
	if err := writeTreeBlob(t.Context(), root, strings.NewReader("new"), entry, copyTreeBlob); err != nil {
		t.Fatal(err)
	}
	info, err := root.Stat(entry.path)
	if err != nil || info.Size() != 3 {
		t.Fatalf("executable = %v, %v", info, err)
	}
	if err := writeTree(t.Context(), MaterializeInput{}, root,
		[]treeEntry{{path: "external", mode: "160000"}}, copyTreeBlob); err != nil {
		t.Fatal(err)
	}
	if err := writeTree(t.Context(), MaterializeInput{}, root,
		[]treeEntry{{path: "value/blocked", mode: "160000"}}, copyTreeBlob); err == nil {
		t.Fatal("file used as directory")
	}
}

func TestTreePayloadProcessFailure(t *testing.T) {
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	dir := t.TempDir()
	object := strings.Repeat("a", 40)
	runnertest.Install(t, dir, "git", runnertest.Behavior{Stdout: object + " blob 0\n\n", Code: 7})
	t.Setenv("PATH", dir)
	err := writeTree(t.Context(), MaterializeInput{}, payloadRoot(t),
		[]treeEntry{{path: "empty", mode: "100644", object: object}}, copyTreeBlob)
	if err == nil || !strings.Contains(err.Error(), "exited 7") {
		t.Fatalf("payload process = %v", err)
	}
}

func TestMaterializeRejectsInvalidBoundsBeforeCreatingResources(t *testing.T) {
	t.Parallel()
	for _, bounds := range []Bounds{{}, {Bytes: -1, Files: 1}, {Bytes: 1, Files: -1}} {
		parent := filepath.Join(t.TempDir(), "absent")
		tree, err := Materialize(t.Context(), MaterializeInput{Parent: parent, Bounds: bounds})
		if tree != nil || !errors.Is(err, model.ErrCannotJudge) {
			t.Fatalf("invalid bounds = %v, %v", tree, err)
		}
		if _, err := os.Stat(parent); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("parent created: %v", err)
		}
	}
	var dst bytes.Buffer
	if err := copyTreeBlob(t.Context(), &dst, strings.NewReader(""), 0); err != nil || dst.Len() != 0 {
		t.Fatalf("empty blob = %v, %v", dst.Bytes(), err)
	}
}
