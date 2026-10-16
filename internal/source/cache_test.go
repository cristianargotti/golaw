package source_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// TestCacheAsksEachDirectoryOnce holds a discovery to what git said the
// first time: the directory asked again and the root it found answer the
// same repository after the repository is gone, an error is kept as well,
// and a reset asks git again.
func TestCacheAsksEachDirectoryOnce(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "dir/a.txt", "a\n")
	cache := source.NewCache()
	first, err := cache.Discover(t.Context(), filepath.Join(root, "dir"))
	if err != nil || first.Root != root {
		t.Fatalf("Discover = %+v, %v", first, err)
	}
	outside := t.TempDir()
	if _, err := cache.Discover(t.Context(), outside); !errors.Is(err, source.ErrNotRepository) {
		t.Fatalf("outside git: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{filepath.Join(root, "dir"), root} {
		if again, err := cache.Discover(t.Context(), dir); err != nil || again != first {
			t.Fatalf("%s asked again = %+v, %v", dir, again, err)
		}
	}
	if _, err := cache.Discover(t.Context(), outside); !errors.Is(err, source.ErrNotRepository) {
		t.Fatalf("a kept error: %v", err)
	}
	cache.Reset()
	if _, err := cache.Discover(t.Context(), root); err == nil {
		t.Fatal("a reset cache answered a repository that is gone")
	}
}

// TestCacheListsEachWorktreeOnce holds the universe to the first listing
// of a repository: a later snapshot reads a changed file again, keeps a
// file created since out and still answers a deleted one as gone.
func TestCacheListsEachWorktreeOnce(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "a.txt", "one\n")
	write(t, root, "b.txt", "b\n")
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	cache := source.NewCache()
	first, err := cache.Worktree(t.Context(), repo)
	if err != nil || mustRead(t, first, "a.txt") != "one\n" {
		t.Fatalf("first listing: %v", err)
	}
	write(t, root, "a.txt", "two\n")
	write(t, root, "late.txt", "late\n")
	if removeErr := os.Remove(filepath.Join(root, "b.txt")); removeErr != nil {
		t.Fatal(removeErr)
	}
	later, err := cache.Worktree(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if mustRead(t, later, "a.txt") != "two\n" || later.Class("late.txt") != model.ClassUnknown || later.Class("b.txt") != model.ClassUnknown {
		t.Fatal("a later snapshot must read again within the first listing")
	}
	if mustRead(t, first, "a.txt") != "one\n" {
		t.Fatal("the first snapshot changed")
	}
	if _, listErr := cache.Worktree(t.Context(), source.Repo{Root: filepath.Join(root, "absent")}); listErr == nil {
		t.Fatal("a repository that cannot be listed was answered")
	}
}

// TestWorktreeSelectsOnlyWhatItIsAskedFor answers the files of a part of
// the universe in path order, presence without a read, and leaves a file
// it was not asked for unread, so a change to it shows on its first read.
func TestWorktreeSelectsOnlyWhatItIsAskedFor(t *testing.T) {
	t.Parallel()
	root, repo := classes(t)
	w, err := source.NewWorktree(t.Context(), repo)
	if err != nil {
		t.Fatal(err)
	}
	selected := w.Select(func(p string) bool { return filepath.Ext(p) == ".go" })
	paths := make([]string, 0, len(selected))
	for _, f := range selected {
		paths = append(paths, f.Path)
	}
	if !slices.Equal(paths, []string{"a.go", "a_test.go", "gen.go", "untracked.go"}) {
		t.Fatalf("Select = %v", paths)
	}
	if !w.Any(func(p string) bool { return p == "testdata/f.txt" }) || w.Any(func(p string) bool { return p == "ignored.log" }) {
		t.Fatal("Any answers the universe")
	}
	if err := os.Remove(filepath.Join(root, "empty.txt")); err != nil {
		t.Fatal(err)
	}
	if w.Any(func(p string) bool { return p == "empty.txt" }) || w.Class("empty.txt") != model.ClassUnknown {
		t.Fatal("a listed file deleted before its first read is gone")
	}
	if w.Class("a.go") != model.ClassTracked || !w.Any(func(p string) bool { return p == "a.go" }) {
		t.Fatal("an inspected file stays present")
	}
}

// TestChangesNamesWhatDiffersFromTheIndex lists what git status reports
// against HEAD and the index, a deleted file included, as a snapshot that
// reads those paths only, and leaves a clean file and an ignored one out.
func TestChangesNamesWhatDiffersFromTheIndex(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, ".gitignore", "*.log\n")
	for _, name := range []string{"a.txt", "c.txt", "clean.txt", "d.txt"} {
		write(t, root, name, "one\n")
	}
	commitAll(t, root, "one")
	write(t, root, "a.txt", "two\n")
	write(t, root, "b.txt", "new\n")
	write(t, root, "skip.log", "ignored\n")
	write(t, root, "d.txt", "staged\n")
	gitRun(t, root, "add", "d.txt")
	if err := os.Remove(filepath.Join(root, "c.txt")); err != nil {
		t.Fatal(err)
	}
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	st, err := source.Changes(t.Context(), repo, nil)
	if err != nil || !slices.Equal(st.Paths, []string{"a.txt", "b.txt", "c.txt", "d.txt"}) {
		t.Fatalf("Changes = %v, %v", st.Paths, err)
	}
	snapshot := st.Worktree
	if mustRead(t, snapshot, "a.txt") != "two\n" || snapshot.Class("c.txt") != model.ClassUnknown || snapshot.Class("clean.txt") != model.ClassUnknown {
		t.Fatal("the snapshot of the changes reads the changed paths only")
	}
	blob := strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD:a.txt"))
	if st.Heads["a.txt"] != blob || st.Heads["c.txt"] != blob || st.Heads["b.txt"] != "" {
		t.Fatalf("the blobs HEAD holds = %v, want %s for a.txt and c.txt only", st.Heads, blob)
	}
	beside, err := source.Changes(t.Context(), repo, []string{"clean.txt", "gone.txt"})
	if err != nil || mustRead(t, beside.Worktree, "clean.txt") != "one\n" || beside.Worktree.Class("gone.txt") != model.ClassUnknown || slices.Contains(beside.Paths, "clean.txt") {
		t.Fatalf("the paths asked for beside the changes: %v, %v", beside.Paths, err)
	}
	if _, err := source.Changes(t.Context(), source.Repo{Root: t.TempDir()}, nil); err == nil {
		t.Fatal("a directory outside git was listed")
	}
}
