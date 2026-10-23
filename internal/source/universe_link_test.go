package source_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// TestWorktreeSizesALinkByItsText holds the size of a link in the
// worktree to the length of its text, the size git records for its blob,
// on every system: Unix lstat answers that length while Windows answers
// zero for every link, and change detection tells two texts of one link
// apart by that size alone.
func TestWorktreeSizesALinkByItsText(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "a.txt", "a\n")
	if err := os.Symlink("a.txt", filepath.Join(root, "link")); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	commitAll(t, root, "one")
	for _, text := range []string{"a.txt", "a-longer-text.txt"} {
		if err := os.Remove(filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(text, filepath.Join(root, "link")); err != nil {
			t.Fatal(err)
		}
		if got := linkFile(t, root); got.Class != model.ClassSymlink || got.Size != int64(len(text)) {
			t.Fatalf("a link to %q = %+v, want the size %d", text, got, len(text))
		}
	}
}

// linkFile is the entry of the path link in the worktree universe.
func linkFile(t *testing.T, root string) model.File {
	t.Helper()
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	w, err := source.NewWorktree(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range w.Files() {
		if file.Path == "link" {
			return file
		}
	}
	t.Fatalf("no link in %+v", w.Files())
	return model.File{}
}
