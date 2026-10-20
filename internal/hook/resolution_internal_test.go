package hook

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestSameLinesIgnoresTheNamesOfTheSides compares a text on disk with the
// one git made: the same lines with another line end, and conflict markers
// whose sides git named otherwise, are the same; another line, another
// count of lines, a marker of another length or character, a quote line
// that only looks like a marker, and names compared without markers are
// not.
func TestSameLinesIgnoresTheNamesOfTheSides(t *testing.T) {
	t.Parallel()
	git := "<<<<<<< c1\nmine\n||||||| base\nstart\n=======\ntheirs\n>>>>>>> m1\n"
	for _, tc := range []struct {
		disk, git string
		markers   bool
		same      bool
	}{
		{git, git, true, true},
		{"<<<<<<< HEAD\nmine\n||||||| 5cd13f6\nstart\n=======\ntheirs\n>>>>>>> side\n", git, true, true},
		{"<<<<<<< HEAD\r\nmine\r\n||||||| base\r\nstart\r\n=======\r\ntheirs\r\n>>>>>>> side\r\n", git, true, true},
		{"one\r\ntwo\r\n", "one\ntwo\n", false, true},
		{"<<<<<<< HEAD\nmine\n||||||| base\nstart\n=======\ntheirs\n>>>>>>> side\n", git, false, false},
		{"<<<<<<< HEAD\nmine, changed\n||||||| base\nstart\n=======\ntheirs\n>>>>>>> side\n", git, true, false},
		{"<<<<<<<< HEAD\nmine\n||||||| base\nstart\n=======\ntheirs\n>>>>>>> side\n", git, true, false},
		{"<<<<<<< HEAD\nmine\n||||||| base\nstart\n=======\ntheirs\n<<<<<<< side\n", git, true, false},
		{"> mine\n", "> theirs\n", true, false},
		{"<<<<<<<<x\n", "<<<<<<<<y\n", true, false},
		{"one\n", "one\ntwo\n", true, false},
	} {
		if got := sameLines([]byte(tc.disk), []byte(tc.git), tc.markers); got != tc.same {
			t.Errorf("%q against %q with markers %v: same %v", tc.disk, tc.git, tc.markers, got)
		}
	}
}

// conflictsOf answers the bytes the merge of c1 and m1, the commit m1 and
// the commit c1 hold at each path asked, recording each question: every
// conflicted file holds the teammate's dash in its markers and a legacy
// dash at c1, over.md is over the read bound in all three, and unknown.md
// is left out, as a name git cannot carry.
func conflictsOf(asked *[]string) func(context.Context, Repository, string, string, []string, []string) (map[string]Conflict, error) {
	return func(_ context.Context, _ Repository, ours, theirs string, options, paths []string) (map[string]Conflict, error) {
		*asked = append(*asked, ours+".."+theirs+" "+strings.Join(options, ",")+" "+strings.Join(paths, ","))
		conflict := Conflict{
			Merged: []byte("<<<<<<< c1\nmine \u2014 old\n=======\ntheirs \u2014 x\n>>>>>>> m1\n"),
			Theirs: []byte("theirs \u2014 x\n"),
			Ours:   []byte("mine \u2014 old\n"),
		}
		return map[string]Conflict{
			"conflict.md": conflict, "crlf.md": conflict, "gone.md": conflict, "mine.md": conflict, "ours.md": conflict, "theirs.md": conflict, "over.md": {},
		}, nil
	}
}

// resolutionChanges are the changes git status reports after a command
// merged m1 into c1 and left it in progress: the unmerged paths and the
// bytes on disk of each but gone.md, deleted, huge.md over the read bound.
func resolutionChanges() Changes {
	return Changes{
		Head: "c1", Merge: "m1",
		Snapshot: proof.NewTree(map[string][]byte{
			"conflict.md": []byte("<<<<<<< HEAD\nmine \u2014 old\n=======\ntheirs \u2014 x\n>>>>>>> side\n"),
			"crlf.md":     []byte("<<<<<<< HEAD\r\nmine \u2014 old\r\n=======\r\ntheirs \u2014 x\r\n>>>>>>> side\r\n"),
			"huge.md":     append(bytes.Repeat([]byte("x"), model.MaxReadBytes), "\nmine \u2014 resolved\n"...),
			"mine.md":     []byte("mine \u2014 resolved\n"),
			"ours.md":     []byte("mine \u2014 old\r\n"),
			"theirs.md":   []byte("theirs \u2014 x\r\n"),
			"over.md":     []byte("anything\n"),
			"unknown.md":  []byte("mine \u2014 resolved\n"),
		}),
		Unmerged: map[string]bool{
			"conflict.md": true, "crlf.md": true, "gone.md": true, "huge.md": true, "mine.md": true, "ours.md": true, "over.md": true,
			"theirs.md": true, "unknown.md": true,
		},
	}
}

// TestResolutionsJudgeWhatTheCommandWrote judges, of the paths a merge the
// command started left unmerged, the one whose bytes the command wrote
// without git add, names the one that holds the merged commit's bytes as
// brought, keeps silent the one that holds the bytes of HEAD, unchanged,
// and leaves to the event of its resolution each that holds the conflict
// git wrote, whatever its line ends and the names of its sides, one gone
// from disk, one over the read bound, one whose bytes git could not read
// and one git could not name; git is asked once without an option, only for
// the paths whose bytes were read, and once more with the strategy option
// of the command for the bytes that merge left unmatched, never for a merge
// the command did not start, an octopus merge, without a binding or when no
// path could be read, and when git cannot answer every path waits.
func TestResolutionsJudgeWhatTheCommandWrote(t *testing.T) {
	t.Parallel()
	unmerged := []string{"conflict.md", "crlf.md", "gone.md", "huge.md", "mine.md", "ours.md", "over.md", "theirs.md", "unknown.md"}
	run := execution{repository: Repository{Head: "c1"}}
	var asked []string
	deps := Dependencies{Conflicts: conflictsOf(&asked)}
	before := Baseline{Files: map[string]string{}, Head: "c1", Options: []string{"theirs"}}
	changed := append(slices.Clone(unmerged), "edited.md")
	got, gaps := run.fromMerge(t.Context(), deps, before, resolutionChanges(), changed)
	waiting := pathGaps([]string{"conflict.md", "crlf.md", "gone.md", "huge.md", "over.md", "unknown.md"}, causeUnmerged)
	read := []string{"conflict.md", "crlf.md", "mine.md", "ours.md", "over.md", "theirs.md", "unknown.md"}
	if !slices.Equal(got, []string{"mine.md", "edited.md"}) || !slices.Equal(gaps, append(waiting, pathGaps([]string{"theirs.md"}, causeMerged)...)) ||
		!slices.Equal(asked, []string{"c1..m1  " + strings.Join(read, ","), "c1..m1 theirs mine.md"}) {
		t.Fatalf("judged %v, gaps %+v after %q", got, gaps, asked)
	}
	asked = nil
	unread := []string{"gone.md", "huge.md"}
	if written, waited := run.resolutions(t.Context(), deps, before, resolutionChanges(), unread); len(written) != 0 ||
		!slices.Equal(waited, pathGaps(unread, causeUnmerged)) || len(asked) != 0 {
		t.Fatalf("no path read: written %v, gaps %+v after %q", written, waited, asked)
	}
	failing := Dependencies{Conflicts: func(context.Context, Repository, string, string, []string, []string) (map[string]Conflict, error) {
		return nil, model.ErrUnreadable
	}}
	octopus := resolutionChanges()
	octopus.Merge = "m1\nm2"
	started := before
	started.Merge = "m1"
	for name, tc := range map[string]struct {
		deps    Dependencies
		before  Baseline
		changes Changes
	}{
		"a merge in progress before the command": {deps, started, resolutionChanges()},
		"an octopus merge":                       {deps, before, octopus},
		"git cannot answer":                      {failing, before, resolutionChanges()},
		"no binding":                             {Dependencies{}, before, resolutionChanges()},
	} {
		asked = nil
		written, waited := run.resolutions(t.Context(), tc.deps, tc.before, tc.changes, unmerged)
		if len(written) != 0 || !slices.Equal(waited, pathGaps(unmerged, causeUnmerged)) || len(asked) != 0 {
			t.Fatalf("%s: written %v, gaps %+v after %q", name, written, waited, asked)
		}
	}
}

// optionConflicts answer, at each path asked, the conflict the merge of c1
// and m1 holds without an option, in its markers, and the bytes the merge
// with an option holds, recording each question; failing makes the merge
// with an option fail.
func optionConflicts(asked *[]string, failing bool) func(context.Context, Repository, string, string, []string, []string) (map[string]Conflict, error) {
	return func(_ context.Context, _ Repository, _, _ string, options, paths []string) (map[string]Conflict, error) {
		*asked = append(*asked, strings.Join(options, ",")+" "+strings.Join(paths, ","))
		if len(options) > 0 && failing {
			return nil, model.ErrUnreadable
		}
		merged := "<<<<<<< c1\nlocal\n=======\nmate \u2014 line\n>>>>>>> m1\n"
		if len(options) > 0 {
			merged = "as the option resolved it\n"
		}
		conflicts := map[string]Conflict{}
		for _, path := range paths {
			conflicts[path] = Conflict{Merged: []byte(merged), Theirs: []byte("mate \u2014 line\n"), Ours: []byte("local\n")}
		}
		return conflicts, nil
	}
}

// TestResolutionsCompareEveryMergeTheTextMayHaveMade compares the bytes of
// an unmerged path with the merge without an option and with each strategy
// option of the command apart, because the text does not tell which of its
// merges stopped: the conflict of the merge without an option waits though
// the text names -X theirs, as when a git merge -X theirs after it failed,
// the bytes the merge with -X theirs holds wait too, asked only for the
// bytes the first merge left unmatched, bytes neither holds are judged, and
// when git cannot make the merge with the option they wait.
func TestResolutionsCompareEveryMergeTheTextMayHaveMade(t *testing.T) {
	t.Parallel()
	run := execution{repository: Repository{Head: "c1"}}
	before := Baseline{Files: map[string]string{}, Head: "c1", Options: []string{"theirs"}}
	changes := Changes{
		Head: "c1", Merge: "m1", Unmerged: map[string]bool{"mine.md": true, "option.md": true, "plain.md": true},
		Snapshot: proof.NewTree(map[string][]byte{
			"mine.md":   []byte("mine \u2014 resolved\n"),
			"option.md": []byte("as the option resolved it\n"),
			"plain.md":  []byte("<<<<<<< HEAD\nlocal\n=======\nmate \u2014 line\n>>>>>>> side\n"),
		}),
	}
	unmerged := []string{"mine.md", "option.md", "plain.md"}
	var asked []string
	written, gaps := run.resolutions(t.Context(), Dependencies{Conflicts: optionConflicts(&asked, false)}, before, changes, unmerged)
	if !slices.Equal(written, []string{"mine.md"}) || !slices.Equal(gaps, pathGaps([]string{"option.md", "plain.md"}, causeUnmerged)) ||
		!slices.Equal(asked, []string{" mine.md,option.md,plain.md", "theirs mine.md,option.md"}) {
		t.Fatalf("written %v, gaps %+v after %q", written, gaps, asked)
	}
	asked = nil
	written, gaps = run.resolutions(t.Context(), Dependencies{Conflicts: optionConflicts(&asked, true)}, before, changes, unmerged)
	if len(written) != 0 || !slices.Equal(gaps, pathGaps(unmerged, causeUnmerged)) || len(asked) != 2 {
		t.Fatalf("the merge with the option failed: written %v, gaps %+v after %q", written, gaps, asked)
	}
}
