package source

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestWorktreeKeepsTheLargeFileStat binds the stamp to the first inspection,
// even when the path changes or disappears before the stamp is requested.
func TestWorktreeKeepsTheLargeFileStat(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	name := filepath.Join(root, "large.sql")
	sizedFile(t, name, model.MaxReadBytes+1)
	w := newWorktree(root, []entry{{path: "large.sql", tracked: true}})
	first := worktreeStat(t, w, "large.sql")
	if first == "" || worktreeStat(t, w, "large.sql") != first || worktreeStat(t, w.Refreshed(), "large.sql") != first {
		t.Fatalf("one unchanged file must keep a nonempty stamp: %q", first)
	}
	inspected := w.Refreshed()
	if inspected.Class("large.sql") != model.ClassTracked {
		t.Fatal("the large file must be classified without a read")
	}
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	if worktreeStat(t, w, "large.sql") != first || worktreeStat(t, inspected, "large.sql") != first {
		t.Fatal("the stamp must come from the cached inspection, not another stat")
	}
	if data, err := w.Read("large.sql"); data != nil || !errors.Is(err, model.ErrTooLarge) {
		t.Fatalf("the cached large file holds no bytes: %q, %v", data, err)
	}
	if got := worktreeStat(t, w.Refreshed(), "large.sql"); got != "" {
		t.Fatalf("a missing file has no stamp: %q", got)
	}
}

// TestWorktreeStatFollowsLargeFileChanges distinguishes a replacement, a
// modification time change and growth from the same inspected file.
func TestWorktreeStatFollowsLargeFileChanges(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*testing.T, string)
	}{
		{"replacement", replaceSizedFile},
		{"modification time", touchSizedFile},
		{"growth", growSizedFile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			name := filepath.Join(root, "large.sql")
			sizedFile(t, name, model.MaxReadBytes+1)
			w := newWorktree(root, []entry{{path: "large.sql", tracked: true}})
			before := worktreeStat(t, w, "large.sql")
			tc.change(t, name)
			after := worktreeStat(t, w.Refreshed(), "large.sql")
			if before == "" || after == "" || after == before || worktreeStat(t, w, "large.sql") != before {
				t.Fatalf("the changed file needs a new stamp while its old snapshot stays frozen: %q, %q", before, after)
			}
		})
	}
}

// TestWorktreeStatOnlyNamesLargeRegularFiles leaves readable and unlisted
// files without a stamp and never follows a file or directory link.
func TestWorktreeStatOnlyNamesLargeRegularFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	name := filepath.Join(root, "small.sql")
	sizedFile(t, name, 1)
	w := newWorktree(root, []entry{{path: "small.sql", tracked: true}, {path: "nested", dir: true}})
	for _, path := range []string{"small.sql", "absent.sql", "nested"} {
		if got := worktreeStat(t, w, path); got != "" {
			t.Errorf("StatIdentity(%s) = %q, want none", path, got)
		}
	}
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on Windows")
	}
	outside := t.TempDir()
	sizedFile(t, filepath.Join(outside, "large.sql"), model.MaxReadBytes+1)
	if err := os.Symlink(filepath.Join(outside, "large.sql"), filepath.Join(root, "linked.sql")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	w = newWorktree(root, []entry{{path: "linked.sql", tracked: true}, {path: "linked/large.sql", tracked: true}})
	for _, path := range []string{"linked.sql", "linked/large.sql"} {
		if got := worktreeStat(t, w, path); got != "" {
			t.Errorf("StatIdentity(%s) = %q: a link must never be followed", path, got)
		}
	}
}

// worktreeStat asks the worktree for its optional cached stat identity.
func worktreeStat(t *testing.T, w *Worktree, path string) string {
	t.Helper()
	stamped, ok := any(w).(interface{ StatIdentity(path string) string })
	if !ok {
		t.Fatal("the worktree does not expose its cached stat identity")
	}
	return stamped.StatIdentity(path)
}

// sizedFile creates a sparse file whose size needs no large allocation.
func sizedFile(t *testing.T, name string, size int64) {
	t.Helper()
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
}

// replaceSizedFile replaces a file with another inode of the same size.
func replaceSizedFile(t *testing.T, name string) {
	t.Helper()
	sizedFile(t, name+".new", model.MaxReadBytes+1)
	if err := os.Rename(name+".new", name); err != nil {
		t.Fatal(err)
	}
}

// touchSizedFile changes only the requested modification and access times.
func touchSizedFile(t *testing.T, name string) {
	t.Helper()
	info, err := os.Lstat(name)
	if err != nil {
		t.Fatal(err)
	}
	when := info.ModTime().Add(time.Hour)
	if err := os.Chtimes(name, when, when); err != nil {
		t.Fatal(err)
	}
}

// growSizedFile grows an existing large file by one byte.
func growSizedFile(t *testing.T, name string) {
	t.Helper()
	if err := os.Truncate(name, model.MaxReadBytes+2); err != nil {
		t.Fatal(err)
	}
}
