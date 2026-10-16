package source_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/source"
)

// TestMovedAnswersWhatTwoCommitsHoldApart asks git once for the paths that
// differ between two commits, with the blob the earlier one held at each: a
// file it changed answers its old blob, a file it added none, a file it
// deleted its old blob, a path git would quote is read whole,
// two equal commits answer nothing, and a directory outside git is an
// error.
func TestMovedAnswersWhatTwoCommitsHoldApart(t *testing.T) {
	t.Parallel()
	root := initRepo(t)
	write(t, root, "changed.txt", "one\n")
	write(t, root, "deleted.txt", "two\n")
	write(t, root, "same.txt", "three\n")
	commitAll(t, root, "one")
	head := func(path string) string { return strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD:"+path)) }
	old := map[string]string{"changed.txt": head("changed.txt"), "deleted.txt": head("deleted.txt")}
	from := strings.TrimSpace(gitRun(t, root, "rev-parse", "HEAD"))
	write(t, root, "changed.txt", "four\n")
	write(t, root, "odd name \u00e9.txt", "five\n")
	gitRun(t, root, "rm", "-q", "deleted.txt")
	commitAll(t, root, "two")
	repo, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	moved, err := source.Moved(t.Context(), repo, from, repo.Head)
	if err != nil || len(moved) != 3 || moved["changed.txt"] != old["changed.txt"] || moved["deleted.txt"] != old["deleted.txt"] {
		t.Fatalf("Moved = %v, %v", moved, err)
	}
	if blob, held := moved["odd name \u00e9.txt"]; !held || blob != "" {
		t.Fatalf("an added path = %q, %v", blob, held)
	}
	if none, err := source.Moved(t.Context(), repo, repo.Head, repo.Head); err != nil || len(none) != 0 {
		t.Fatalf("one commit against itself = %v, %v", none, err)
	}
	if _, err := source.Moved(t.Context(), source.Repo{Root: t.TempDir()}, from, from); err == nil {
		t.Fatal("a directory outside git answered a move")
	}
}
