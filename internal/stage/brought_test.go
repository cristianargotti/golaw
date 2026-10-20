package stage_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/stage"
)

// nine is a file of nine numbered lines, so two sides can edit apart.
const nine = "1\n2\n3\n4\n5\n6\n7\n8\n9\n"

// edited is nine with each numbered line replaced by its text.
func edited(lines map[int]string) string {
	out := strings.Split(strings.TrimSuffix(nine, "\n"), "\n")
	for number, text := range lines {
		out[number-1] = text
	}
	return strings.Join(out, "\n") + "\n"
}

// merging is a repository with a merge of side in progress: the side edits
// shared.md and adds theirs.md, our branch edits shared.md as ours says, and
// the merge runs with the options.
func merging(t *testing.T, ours, theirs map[int]string, options ...string) *gittest.Repo {
	t.Helper()
	r := gittest.New(t)
	r.Commit("docs: base", map[string]string{"shared.md": nine})
	r.Branch("side", map[string]string{"shared.md": edited(theirs), "theirs.md": "t " + dash + " t\n"})
	r.Commit("docs: ours", map[string]string{"shared.md": edited(ours), "ours.md": "o\n"})
	r.MergeInProgress("side", options...)
	return r
}

// brought is what Brought names for the index of a repository.
func brought(t *testing.T, r *gittest.Repo) []string {
	t.Helper()
	in := input(t, r, source.IndexOptions{})
	index, err := source.NewIndex(t.Context(), in.Repo, in.Index)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := stage.Brought(t.Context(), in.Repo, index)
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func TestBroughtNamesWhatTheMergeTookFromTheOtherSide(t *testing.T) {
	t.Parallel()
	both := []string{"shared.md", "theirs.md"}
	cases := map[string]struct {
		r    *gittest.Repo
		want []string
	}{
		"a file only the side changed": {merging(t, map[int]string{}, map[int]string{9: "9 " + dash}), both},
		"hunks git merged by itself":   {merging(t, map[int]string{1: "1 ours"}, map[int]string{9: "9 " + dash}), both},
		"a conflict -X theirs resolved": {
			merging(t, map[int]string{1: "1 ours", 9: "9 ours"}, map[int]string{1: "1 theirs " + dash}, "-X", "theirs"), both,
		},
		"a conflict -X ours resolved": {
			merging(t, map[int]string{1: "1 ours"}, map[int]string{1: "1 theirs", 9: "9 " + dash}, "-X", "ours"), both,
		},
	}
	for name, tc := range cases {
		if got := brought(t, tc.r); !slices.Equal(got, tc.want) {
			t.Errorf("%s: brought %v, want %v", name, got, tc.want)
		}
		res := scan(t, tc.r, source.IndexOptions{})
		if got := findingPaths(res.Decision); len(got) != 0 || !slices.Equal(res.Brought, tc.want) {
			t.Errorf("%s: findings at %v, brought %v", name, got, res.Brought)
		}
		took := func(path string) bool { return slices.Contains(tc.want, path) }
		if slices.ContainsFunc(res.Input.Targets, func(f model.File) bool { return took(f.Path) }) ||
			slices.ContainsFunc(res.Input.Changes, func(c model.Change) bool { return took(c.NewPath) }) {
			t.Errorf("%s: the input keeps a brought path: %+v, %+v", name, res.Input.Targets, res.Input.Changes)
		}
	}
}

// What the other side of a merge holds is what the merge brought, so a
// directory finding the file it brought makes is left to the pipeline with
// the file, an edit of the commit's own beside it or not, while a file of
// the commit's own that crosses the limit is judged.
func TestBroughtLeavesTheDirectoryFindingOfTheOtherSide(t *testing.T) {
	t.Parallel()
	theirs := at(adapterDir+"/f13.go", "package http\n")
	cases := map[string]struct {
		side, own     map[string]string
		want, brought []string
	}{
		"nothing of its own":         {theirs, nil, nil, []string{adapterDir + "/f13.go"}},
		"an own edit beside it":      {theirs, at(adapterDir+"/f01.go", "package http\n\nvar x = 1\n"), nil, []string{adapterDir + "/f13.go"}},
		"an own file over the limit": {at("other.md", "other\n"), theirs, []string{adapterDir}, []string{"other.md"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := goRepo(t, adapter(12), nil)
			r.Branch("side", tc.side)
			r.MergeInProgress("side")
			r.Stage(tc.own)
			res := scan(t, r, source.IndexOptions{})
			if got := findingPaths(res.Decision); !slices.Equal(got, tc.want) || !slices.Equal(res.Brought, tc.brought) {
				t.Errorf("findings at %v, brought %v; want %v and %v", got, res.Brought, tc.want, tc.brought)
			}
		})
	}
}

// A finding both parents of a merge hold is held once, so a second one the
// commit adds beside it is its own.
func TestBroughtCountsAFindingBothSidesHoldOnce(t *testing.T) {
	t.Parallel()
	held := "x " + dash + " y\n"
	r := gittest.New(t)
	r.Commit("docs: base", map[string]string{"shared.md": held})
	r.Branch("side", map[string]string{"side.md": "s\n"})
	r.Commit("docs: ours", map[string]string{"ours.md": "o\n"})
	r.MergeInProgress("side")
	r.Stage(map[string]string{"shared.md": held + held})
	res := scan(t, r, source.IndexOptions{})
	if got := findingPaths(res.Decision); !slices.Equal(got, []string{"shared.md"}) || !slices.Equal(res.Brought, []string{"side.md"}) {
		t.Errorf("findings at %v, brought %v; want the second dash of shared.md and side.md", got, res.Brought)
	}
}

// The other parent left a file over the read bound, so a finding the
// commit keeps there may be one the other side holds: it is held back and
// named, never reported as the commit's own.
func TestBroughtHoldsBackWhatAnUnreadOtherSideCannotDisprove(t *testing.T) {
	t.Parallel()
	short := "one " + dash + " two\n"
	r := gittest.New(t)
	r.Commit("docs: base", map[string]string{"big.md": "clean\n"})
	r.Branch("side", map[string]string{"big.md": short + strings.Repeat("a\n", model.MaxReadBytes/2)})
	r.Commit("docs: ours", map[string]string{"ours.md": "o\n"})
	r.MergeInProgress("side")
	r.Stage(map[string]string{"big.md": short})
	res := scan(t, r, source.IndexOptions{})
	dashes := outcomeOf(t, res, "text.dashes")
	if got := findingPaths(res.Decision); len(got) != 0 || !slices.Equal(dashes.Unread, []string{"big.md"}) ||
		!strings.Contains(dashes.Status.Reason, "compare with the base: read big.md") {
		t.Errorf("findings at %v, text.dashes %+v", got, dashes.Status)
	}
}

func TestBroughtLeavesTheCommitsOwnResolutionJudged(t *testing.T) {
	t.Parallel()
	r := merging(t, map[int]string{1: "1 ours"}, map[int]string{1: "1 theirs"})
	r.Stage(map[string]string{"shared.md": edited(map[int]string{1: "1 mine " + dash}), "own.md": "own " + dash + " own\n"})
	if got := brought(t, r); !slices.Equal(got, []string{"theirs.md"}) {
		t.Errorf("brought %v, want theirs.md alone", got)
	}
	if got := findingPaths(scan(t, r, source.IndexOptions{}).Decision); !slices.Equal(got, []string{"own.md", "shared.md"}) {
		t.Errorf("findings at %v, want own.md and shared.md", got)
	}
}

func TestBroughtAsksNothingWithoutAMergeInProgress(t *testing.T) {
	t.Parallel()
	r := gittest.New(t)
	r.Commit("docs: base", map[string]string{"a.md": "a\n"})
	r.Stage(map[string]string{"a.md": "b " + dash + " b\n"})
	if got := brought(t, r); got != nil {
		t.Errorf("brought %v without a merge", got)
	}
	if res := scan(t, r, source.IndexOptions{}); res.Brought != nil || !slices.Equal(findingPaths(res.Decision), []string{"a.md"}) {
		t.Errorf("result %+v", res)
	}
}
