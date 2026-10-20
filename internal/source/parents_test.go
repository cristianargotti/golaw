package source_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/source"
)

// TestParentsNamesTheCommitsACommitWasMadeOn answers the parents of a
// commit in the order git records them, none for a root commit, one for a
// commit on it and two for a merge, the branch merged into first, and git
// log names the same parents beside each commit it lists; a commit git does
// not hold is an error.
func TestParentsNamesTheCommitsACommitWasMadeOn(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	root := r.Commit("docs: start", map[string]string{"a.md": "a\n"})
	side := r.Branch("side", map[string]string{"b.md": "b\n"})
	first := r.Commit("docs: main", map[string]string{"c.md": "c\n"})
	r.Git("merge", "-q", "--no-ff", "-m", "docs: merge", "side")
	merge := r.Head()
	repo, err := source.Discover(t.Context(), r.Root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{root: nil, side: {root}, first: {root}, merge: {first, side}}
	for commit, parents := range want {
		if got, parentsErr := source.Parents(t.Context(), repo, commit); parentsErr != nil || !slices.Equal(got, parents) {
			t.Errorf("parents of %s = %v, %v, want %v", commit, got, parentsErr, parents)
		}
	}
	commits, err := source.Commits(t.Context(), repo, "", merge, 0)
	if err != nil || len(commits) != len(want) {
		t.Fatalf("commits %+v, %v", commits, err)
	}
	for _, commit := range commits {
		if !slices.Equal(commit.Parents, want[commit.Name]) || commit.Merge != (len(want[commit.Name]) > 1) {
			t.Errorf("git log named %s with the parents %v", commit.Name, commit.Parents)
		}
	}
	if _, err := source.Parents(t.Context(), repo, strings.Repeat("0", 40)); err == nil {
		t.Fatal("a commit git does not hold answered its parents")
	}
}
