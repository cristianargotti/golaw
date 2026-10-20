package source_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
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

// TestConflictsReadNoBlobOverTheReadBound reads, for a merge whose big.md
// conflicts in a file over the read bound on both sides and whose shared.md
// conflicts within it, the bytes of shared.md and none of big.md: git
// cat-file --batch reads only the blobs within the bound, and runs not at
// all when every blob asked is over it. The test is serial because it
// traces the git commands through the environment.
func TestConflictsReadNoBlobOverTheReadBound(t *testing.T) {
	r := gittest.New(t)
	big := strings.Repeat("common line\n", model.MaxReadBytes/12+1)
	r.Commit("docs: start", map[string]string{"big.md": big + "start\n", "shared.md": "start\n"})
	side := r.Branch("side", map[string]string{"big.md": big + "side\n", "shared.md": "side\n"})
	r.Commit("docs: main", map[string]string{"big.md": big + "main\n", "shared.md": "main\n"})
	repo, err := source.Discover(t.Context(), r.Root)
	if err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	conflicts, err := source.Conflicts(t.Context(), repo, repo.Head, side, nil, []string{"big.md", "shared.md"})
	over := conflicts["big.md"]
	if err != nil || over.Merged != nil || over.Theirs != nil || over.Ours != nil || string(conflicts["shared.md"].Theirs) != "side\n" || batchReads(t, trace) != 1 {
		t.Fatalf("conflicts %d paths, big.md read %v, %v", len(conflicts), over.Merged != nil || over.Theirs != nil || over.Ours != nil, err)
	}
	if writeErr := os.WriteFile(trace, nil, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	only, err := source.Conflicts(t.Context(), repo, repo.Head, side, nil, []string{"big.md"})
	if err != nil || len(only) != 1 || only["big.md"].Merged != nil || batchReads(t, trace) != 0 {
		t.Fatalf("big.md alone: %d paths, %v, %d batch reads", len(only), err, batchReads(t, trace))
	}
}

// batchReads counts the git cat-file --batch commands a trace of git holds,
// the ones that read the bytes of objects rather than describe them.
func batchReads(t *testing.T, trace string) int {
	t.Helper()
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasSuffix(line, "trace: built-in: git cat-file --batch") {
			count++
		}
	}
	return count
}

// TestStyledConflictsWriteTheConflictInTheBaseStyles reads, for the merge
// of main and a side that changed shared.md apart from it on two lines, the
// bytes of the merge git makes by itself in the diff3 style and in the
// zdiff3 style, the base of each conflict between its markers as git
// checkout --conflict writes it but for the names of the sides, nil for a
// path no merge holds; the repository receives no object, and a commit git
// does not hold is an error.
func TestStyledConflictsWriteTheConflictInTheBaseStyles(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("docs: start", map[string]string{"shared.md": "a\nstart\nc\nd\n"})
	side := r.Branch("side", map[string]string{"shared.md": "a\nmate line\nc\nd\nside end\n"})
	r.Commit("docs: main", map[string]string{"shared.md": "a\nlocal line\nc\nd\nmain end\n"})
	repo, err := source.Discover(t.Context(), r.Root)
	if err != nil {
		t.Fatal(err)
	}
	objects := looseObjects(t, r.Root)
	styled, err := source.StyledConflicts(t.Context(), repo, repo.Head, side, []string{"shared.md", "gone.md"})
	base := r.Git("rev-parse", "--short", "HEAD~1")
	want := "a\n<<<<<<< " + repo.Head + "\nlocal line\n||||||| " + base + "\nstart\n=======\nmate line\n>>>>>>> " + side + "\nc\nd\n" +
		"<<<<<<< " + repo.Head + "\nmain end\n||||||| " + base + "\n=======\nside end\n>>>>>>> " + side + "\n"
	if err != nil || len(styled["shared.md"]) != 2 || string(styled["shared.md"][0]) != want || !strings.Contains(string(styled["shared.md"][1]), "||||||| ") ||
		len(styled["gone.md"]) != 2 || styled["gone.md"][0] != nil || styled["gone.md"][1] != nil {
		t.Fatalf("styled %q, %v", styled, err)
	}
	if after := looseObjects(t, r.Root); after != objects {
		t.Fatalf("the merges wrote %d objects into the repository", after-objects)
	}
	if _, unknownErr := source.StyledConflicts(t.Context(), repo, repo.Head, strings.Repeat("0", 40), []string{"shared.md"}); unknownErr == nil {
		t.Fatal("a commit git does not hold answered a merge")
	}
}
