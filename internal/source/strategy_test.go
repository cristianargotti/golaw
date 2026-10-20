package source_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/source"
)

// mixedSides is a repository whose branch side changed the first and the
// fourth line of shared.md while main changed its first and its last line,
// so a merge of side without an option stops on the conflict of the first
// line, -X ours keeps the local first line and -X theirs takes the
// teammate's, and both merge the fourth and the last line by themselves; it
// answers the repository and the commit of side.
func mixedSides(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	r := gittest.New(t)
	r.Commit("docs: start", map[string]string{"shared.md": "1\n2\n3\n4\n5\n6\n7\n"})
	side := r.Branch("side", map[string]string{"shared.md": "theirs 1\n2\n3\ntheirs 4\n5\n6\n7\n"})
	r.Commit("docs: ours", map[string]string{"shared.md": "ours 1\n2\n3\n4\n5\n6\nours 7\n"})
	return r, side
}

// TestAutomaticMergeWithTakesTheStrategyOptions makes the merge of main and
// side with no option, with -X ours and with -X theirs and names, at
// shared.md, the blob of each merge beside the blob side holds: the blob of
// each option is the one git merge stages with the same option, the three
// merges differ, AutomaticMerge answers as no option, the repository
// receives no object, and an option git does not know is an error.
func TestAutomaticMergeWithTakesTheStrategyOptions(t *testing.T) {
	t.Parallel()
	r, side := mixedSides(t)
	repo, err := source.Discover(t.Context(), r.Root)
	if err != nil {
		t.Fatal(err)
	}
	objects := looseObjects(t, r.Root)
	theirs := r.Git("rev-parse", side+":shared.md")
	made := map[string]string{}
	for _, option := range []string{"", "ours", "theirs"} {
		var options []string
		if option != "" {
			options = []string{option}
		}
		blobs, mergeErr := source.AutomaticMergeWith(t.Context(), repo, repo.Head, side, options, []string{"shared.md"})
		if mergeErr != nil || len(blobs["shared.md"]) != 2 || blobs["shared.md"][1] != theirs || blobs["shared.md"][0] == "" {
			t.Fatalf("-X %q: blobs %v, %v", option, blobs, mergeErr)
		}
		made[option] = blobs["shared.md"][0]
	}
	plain, err := source.AutomaticMerge(t.Context(), repo, repo.Head, side, []string{"shared.md"})
	if err != nil || plain["shared.md"][0] != made[""] || made[""] == made["ours"] || made["ours"] == made["theirs"] || made[""] == made["theirs"] {
		t.Fatalf("plain %v, %v, the merges %v", plain, err, made)
	}
	if after := looseObjects(t, r.Root); after != objects {
		t.Fatalf("the merges wrote %d objects into the repository", after-objects)
	}
	if _, err := source.AutomaticMergeWith(t.Context(), repo, repo.Head, side, []string{"no-such-option"}, []string{"shared.md"}); err == nil {
		t.Fatal("an option git does not know answered a merge")
	}
	for _, option := range []string{"ours", "theirs"} {
		w := r.Worktree("merge-" + option)
		w.MergeInProgress("side", "-X", option)
		if staged := w.Git("rev-parse", ":shared.md"); staged != made[option] {
			t.Errorf("-X %s: git merge staged %s, the merge made %s", option, staged, made[option])
		}
	}
}

// TestRemergedWithNamesWhatAMergeHoldsApartFromItsOptionMerge reads merge
// commits against the merge git makes of their parents with a strategy
// option: a merge -X theirs committed holds shared.md apart from the merge
// without an option and from the merge -X ours, and nothing apart from the
// merge -X theirs; a merge whose conflict the command resolved with bytes of
// its own holds the resolution and its new file apart from the merge -X
// theirs; the repository receives no object, and a commit git does not hold
// is an error.
func TestRemergedWithNamesWhatAMergeHoldsApartFromItsOptionMerge(t *testing.T) {
	t.Parallel()
	r, _ := mixedSides(t)
	r.Git("merge", "-q", "--no-edit", "-X", "theirs", "side")
	merge := r.Head()
	repo, err := source.Discover(t.Context(), r.Root)
	if err != nil {
		t.Fatal(err)
	}
	objects := looseObjects(t, r.Root)
	for _, tc := range []struct {
		options, want []string
	}{
		{nil, []string{"shared.md"}},
		{[]string{"ours"}, []string{"shared.md"}},
		{[]string{"theirs"}, nil},
	} {
		remerged, remergeErr := source.RemergedWith(t.Context(), repo, []string{merge}, tc.options)
		if remergeErr != nil || len(remerged) != 1 || !slices.Equal(remerged[merge], tc.want) {
			t.Fatalf("-X %v: remerged %v, %v", tc.options, remerged, remergeErr)
		}
	}
	if after := looseObjects(t, r.Root); after != objects {
		t.Fatalf("the remerges wrote %d objects into the repository", after-objects)
	}
	root, _, resolved := resolvedMerge(t)
	own, err := source.Discover(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if remerged, err := source.RemergedWith(t.Context(), own, []string{resolved}, []string{"theirs"}); err != nil ||
		!slices.Equal(remerged[resolved], []string{"mine.md", "shared.md"}) {
		t.Fatalf("an own resolution: remerged %v, %v", remerged, err)
	}
	if _, err := source.RemergedWith(t.Context(), repo, []string{strings.Repeat("0", 40)}, nil); err == nil {
		t.Fatal("a commit git does not hold answered a remerge")
	}
}
