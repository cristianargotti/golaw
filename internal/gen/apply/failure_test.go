package apply_test

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/source"
)

// A file the plan read and that is gone now is a concurrent edit too: the
// agent deleted it, so the engine never brings it back behind its back.
func TestFilesSkipsAFileDeletedSinceThePlanReadIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writes := []gen.Write{{Path: "plans.go", Content: []byte("stub\n"), Mode: gen.WriteAppend, Base: source.ContentHash([]byte("read\n"))}}
	report, err := apply.Files(root, writes)
	if err != nil || !reflect.DeepEqual(report.Skipped, []apply.Skip{{Path: "plans.go", Cause: apply.CauseChanged}}) {
		t.Fatalf("report = %+v, %v", report, err)
	}
	if _, err := os.Stat(filepath.Join(root, "plans.go")); !os.IsNotExist(err) {
		t.Errorf("a deleted file was written again: %v", err)
	}
}

func TestFilesNamesARootThatCannotOpen(t *testing.T) {
	t.Parallel()
	if _, err := apply.Files(filepath.Join(t.TempDir(), "absent"), nil); err == nil {
		t.Error("an absent root answered no error")
	}
}

// Red row: a directory the engine may not write into stops the run with the
// file named and no temporary file left behind.
func TestFilesLeavesNoTemporaryFileWhenAWriteFails(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("a read only directory mode is not enforced on Windows")
	}
	root := t.TempDir()
	put(t, root, "locked/old.go", "old\n", 0o600)
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(locked, 0o700); err != nil {
			t.Error(err)
		}
	})
	writes := []gen.Write{{Path: "locked/old.go", Content: []byte("new\n"), Mode: gen.WriteRegenerable, Base: source.ContentHash([]byte("old\n"))}}
	report, err := apply.Files(root, writes)
	if err == nil || len(report.Written) != 0 || read(t, root, "locked/old.go") != "old\n" {
		t.Fatalf("report = %+v, %v", report, err)
	}
	if left := leftovers(t, root); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
}
