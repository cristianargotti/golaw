package source_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/source"
)

// TestConflictsReadTheBytesOfTheMergeGitMakes reads, for the merge of main
// and a side that changed shared.md too, the bytes of the merge git makes
// by itself, the conflict of shared.md in its markers as git merge writes
// it but for the names of the sides, beside the bytes side and main hold:
// theirs.md as side brought it, a file only main holds at the merge and at
// main, and, with -X theirs, the line of side where the conflict was; a
// name one line of input cannot carry is left out, the repository receives
// no object, and a commit git does not hold is an error.
func TestConflictsReadTheBytesOfTheMergeGitMakes(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("docs: start", map[string]string{"shared.md": "start\n"})
	side := r.Branch("side", map[string]string{"shared.md": "side\n", "theirs.md": "theirs\n"})
	r.Commit("docs: main", map[string]string{"shared.md": "main\n", "local.md": "local\n"})
	repo, err := source.Discover(t.Context(), r.Root)
	if err != nil {
		t.Fatal(err)
	}
	objects := looseObjects(t, r.Root)
	paths := []string{"shared.md", "theirs.md", "local.md", "line\nbreak.md"}
	conflicts, err := source.Conflicts(t.Context(), repo, repo.Head, side, nil, paths)
	marked := "<<<<<<< " + repo.Head + "\nmain\n=======\nside\n>>>>>>> " + side + "\n"
	shared, theirsFile, local := conflicts["shared.md"], conflicts["theirs.md"], conflicts["local.md"]
	if err != nil || len(conflicts) != 3 || string(shared.Merged) != marked || string(shared.Theirs) != "side\n" || string(shared.Ours) != "main\n" ||
		string(theirsFile.Merged) != "theirs\n" || string(theirsFile.Theirs) != "theirs\n" || theirsFile.Ours != nil ||
		string(local.Merged) != "local\n" || local.Theirs != nil || string(local.Ours) != "local\n" {
		t.Fatalf("conflicts %q, %v", conflicts, err)
	}
	theirs, err := source.Conflicts(t.Context(), repo, repo.Head, side, []string{"theirs"}, []string{"shared.md"})
	if err != nil || string(theirs["shared.md"].Merged) != "side\n" {
		t.Fatalf("with -X theirs: %q, %v", theirs, err)
	}
	if after := looseObjects(t, r.Root); after != objects {
		t.Fatalf("the merge wrote %d objects into the repository", after-objects)
	}
	if _, unknownErr := source.Conflicts(t.Context(), repo, repo.Head, strings.Repeat("0", 40), nil, paths); unknownErr == nil {
		t.Fatal("a commit git does not hold answered a merge")
	}
	r.MergeInProgress("side")
	written, err := os.ReadFile(filepath.Join(r.Root, "shared.md"))
	if err != nil || string(written) != "<<<<<<< HEAD\nmain\n=======\nside\n>>>>>>> side\n" {
		t.Fatalf("git merge wrote %q, %v", written, err)
	}
}
