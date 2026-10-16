package source_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// TestWorktreeDropsADeletedTrackedPath keeps a deletion the next commit
// carries out of the universe: git still lists the tracked path, but the
// working tree holds nothing to read, so it is neither a file nor an
// unreadable one, and no check answers Unmeasured for it.
func TestWorktreeDropsADeletedTrackedPath(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "gone.go", "package gone\n")
	write(t, root, "dir/gone.md", "gone\n")
	write(t, root, "kept.go", "package kept\n")
	commitAll(t, root, "one")
	for _, path := range []string{"gone.go", "dir"} {
		if err := os.RemoveAll(filepath.Join(root, path)); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	w, err := source.NewWorktree(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range w.Files() {
		if f.Path == "gone.go" || f.Path == "dir/gone.md" {
			t.Errorf("a deleted path stayed in the universe: %+v", f)
		}
	}
	if w.Class("gone.go") != model.ClassUnknown || w.Class("kept.go") != model.ClassTracked {
		t.Errorf("Files() = %+v", w.Files())
	}
	if err := readErr(w, "dir/gone.md"); !errors.Is(err, model.ErrNotInSnapshot) {
		t.Errorf("a deleted path read as %v", err)
	}
}
