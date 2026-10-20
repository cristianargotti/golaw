package hook

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestStrategyOptionsReadsOursAndTheirs reads -X ours and -X theirs from a
// command text in the four spellings git takes, quoted or before the
// separator of the next command, and nothing else: another value, the -X
// of another program, an -X after the git command ends and a word that
// only starts like one.
func TestStrategyOptionsReadsOursAndTheirs(t *testing.T) {
	t.Parallel()
	for command, want := range map[string][]string{
		"git merge -q --no-commit -X theirs side":         {"theirs"},
		"git merge -Xours side":                           {"ours"},
		"git pull --strategy-option=theirs origin main":   {"theirs"},
		"git pull --strategy-option ours origin main":     {"ours"},
		`git merge -X "theirs" side && git status`:        {"theirs"},
		"git merge -X 'ours' one; git merge -Xtheirs two": {"ours", "theirs"},
		"git merge --no-edit -X theirs; ls":               {"theirs"},
		"git merge -X patience side":                      nil,
		"tar -X excluded.txt -cf out.tar .":               nil,
		"git merge side":                                  nil,
		"git merge -X":                                    nil,
		"echo -Xtheirs-and-more":                          nil,
		"git merge side && echo -X theirs":                nil,
		"git log -X theirs":                               nil,
		"git -C repo merge -X theirs side":                {"theirs"},
		"git -c merge.ff=false pull -Xours origin main":   {"ours"},
	} {
		if got := strategyOptions(command); !slices.Equal(got, want) {
			t.Errorf("%q: options %q, want %q", command, got, want)
		}
	}
}

// optionMerges answer the merge git makes of c1 and m1 with each set of
// strategy options at the staged paths, recording each question: without
// an option auto.md as git merged it and shared.md in its conflict markers,
// with -X theirs shared.md as that option resolved it, and mine.md, which
// only the local side holds, never; failing makes the merge with an option
// fail.
func optionMerges(asked *[]string, failing bool) Dependencies {
	return Dependencies{AutomaticWith: func(_ context.Context, _ Repository, ours, theirs string, options, paths []string) (map[string][]string, error) {
		*asked = append(*asked, ours+".."+theirs+" "+strings.Join(options, ",")+" "+strings.Join(paths, ","))
		if len(options) == 0 {
			return map[string][]string{"auto.md": {"b-auto", "b-side"}, "mine.md": {"b-local", ""}, "shared.md": {"b-conflict", "b-side"}}, nil
		}
		if failing {
			return nil, model.ErrUnreadable
		}
		return map[string][]string{"mine.md": {"b-local", ""}, "shared.md": {"b-theirs", "b-side"}}, nil
	}}
}

// TestMergeBroughtTakesTheOptionsOfTheCommand names as brought by a merge
// in progress a staged file whose blob the merge -X theirs of the command
// holds, asking git that merge only for the files the merge without an
// option leaves, and judges the local file neither holds; without an option
// in the command text git is asked once, and a merge with the option git
// cannot make leaves what the first merge left to a notice.
func TestMergeBroughtTakesTheOptionsOfTheCommand(t *testing.T) {
	t.Parallel()
	changes := Changes{
		Head: "c1", Merge: "m1", Staged: map[string]bool{"auto.md": true, "mine.md": true, "shared.md": true},
		Index: map[string]string{"auto.md": "b-auto", "mine.md": "b-mine", "shared.md": "b-theirs"},
	}
	changed := []string{"auto.md", "mine.md", "shared.md"}
	run := execution{repository: Repository{Head: "c1"}}
	var asked []string
	before := Baseline{Files: map[string]string{}, Head: "c1", Options: []string{"theirs"}}
	got, gaps := run.fromMerge(t.Context(), optionMerges(&asked, false), before, changes, slices.Clone(changed))
	if !slices.Equal(got, []string{"mine.md"}) || !slices.Equal(gaps, pathGaps([]string{"auto.md", "shared.md"}, causeMerged)) ||
		!slices.Equal(asked, []string{"c1..m1  auto.md,mine.md,shared.md", "c1..m1 theirs mine.md,shared.md"}) {
		t.Fatalf("a merge -X theirs: judged %v, gaps %+v after %q", got, gaps, asked)
	}
	asked = nil
	before.Options = nil
	got, gaps = run.fromMerge(t.Context(), optionMerges(&asked, false), before, changes, slices.Clone(changed))
	if !slices.Equal(got, []string{"mine.md", "shared.md"}) || !slices.Equal(gaps, pathGaps([]string{"auto.md"}, causeMerged)) || len(asked) != 1 {
		t.Fatalf("a merge without an option: judged %v, gaps %+v after %q", got, gaps, asked)
	}
	before.Options = []string{"theirs"}
	got, gaps = run.fromMerge(t.Context(), optionMerges(&asked, true), before, changes, slices.Clone(changed))
	want := append(pathGaps([]string{"mine.md", "shared.md"}, causeMergeUnknown), pathGaps([]string{"auto.md"}, causeMerged)...)
	if len(got) != 0 || !slices.Equal(gaps, want) {
		t.Fatalf("a merge with an option git cannot make: judged %v, gaps %+v", got, gaps)
	}
	m1Only := Dependencies{Automatic: func(context.Context, Repository, string, string, []string) (map[string][]string, error) {
		return map[string][]string{"auto.md": {"b-auto", "b-side"}}, nil
	}}
	got, gaps = run.fromMerge(t.Context(), m1Only, before, changes, slices.Clone(changed))
	want = append(pathGaps([]string{"mine.md", "shared.md"}, causeMergeUnknown), pathGaps([]string{"auto.md"}, causeMerged)...)
	if len(got) != 0 || !slices.Equal(gaps, want) {
		t.Fatalf("a binding without the options: judged %v, gaps %+v", got, gaps)
	}
}

// optionRemerges answer the commits of a move whose command made the merge
// m1 in the second its pre event started, the paths m1 holds apart from the
// merge git makes of its parents without an option and, recording each
// question, with the options asked; failing makes the remerge with an
// option fail.
func optionRemerges(second int64, asked *[]string, failing bool) Dependencies {
	return Dependencies{
		Committer: func(context.Context, Repository) (string, error) { return "me <me@example.com>", nil },
		Commits: func(context.Context, Repository, string, string, int64) ([]Commit, error) {
			return []Commit{{Name: "m1", Committer: "me <me@example.com>", Time: second, Merge: true, Paths: []string{"auto.md", "mine.md", "shared.md"}}}, nil
		},
		Remerged: func(context.Context, Repository, []string) (map[string][]string, error) {
			return map[string][]string{"m1": {"mine.md", "shared.md"}}, nil
		},
		RemergedWith: func(_ context.Context, _ Repository, merges, options []string) (map[string][]string, error) {
			*asked = append(*asked, strings.Join(merges, ",")+" "+strings.Join(options, ","))
			if failing {
				return nil, model.ErrUnreadable
			}
			return map[string][]string{"m1": {"mine.md"}}, nil
		},
	}
}

// TestResolvedRemergesWithTheOptionsOfTheCommand judges, of a merge commit
// the command made with -X theirs, only the file it holds apart from the
// merge git makes with that option too, its own file, and names as arrived
// the file the option resolved; the remerge with the option is asked once,
// never without an option in the command text, and when git cannot make it
// each file still apart is a gap.
func TestResolvedRemergesWithTheOptionsOfTheCommand(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, time.October, 17, 12, 0, 0, 0, time.UTC)
	before := Baseline{Head: "c1", At: started, Options: []string{"theirs"}}
	run := execution{repository: Repository{Head: "m1"}}
	brought := []string{"auto.md", "mine.md", "shared.md"}
	var asked []string
	got, gaps := run.fromCommits(t.Context(), optionRemerges(started.Unix(), &asked, false), before, slices.Clone(brought), brought)
	if !slices.Equal(got, []string{"mine.md"}) || !slices.Equal(gaps, pathGaps([]string{"auto.md", "shared.md"}, causeArrived)) ||
		!slices.Equal(asked, []string{"m1 theirs"}) {
		t.Fatalf("a merge -X theirs committed: judged %v, gaps %+v after %q", got, gaps, asked)
	}
	asked, before.Options = nil, nil
	got, gaps = run.fromCommits(t.Context(), optionRemerges(started.Unix(), &asked, false), before, slices.Clone(brought), brought)
	if !slices.Equal(got, []string{"mine.md", "shared.md"}) || !slices.Equal(gaps, pathGaps([]string{"auto.md"}, causeArrived)) || len(asked) != 0 {
		t.Fatalf("a merge without an option: judged %v, gaps %+v after %q", got, gaps, asked)
	}
	before.Options = []string{"theirs"}
	unbound := optionRemerges(started.Unix(), &asked, false)
	unbound.RemergedWith = nil
	for name, deps := range map[string]Dependencies{
		"git cannot remerge with the option": optionRemerges(started.Unix(), &asked, true),
		"no binding":                         unbound,
	} {
		got, gaps = run.fromCommits(t.Context(), deps, before, slices.Clone(brought), brought)
		want := append(pathGaps([]string{"mine.md", "shared.md"}, causeRemergeUnknown), pathGaps([]string{"auto.md"}, causeArrived)...)
		if len(got) != 0 || !slices.Equal(gaps, want) {
			t.Fatalf("%s: judged %v, gaps %+v", name, got, gaps)
		}
	}
}

// TestTheBaselineKeepsTheStrategyOptions saves the strategy options the
// pre event read with its baseline and loads them back.
func TestTheBaselineKeepsTheStrategyOptions(t *testing.T) {
	t.Parallel()
	store := Store{Dir: t.TempDir(), Now: time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC)}
	if err := store.Save(t.Context(), "key", "input", Baseline{Files: map[string]string{}, Head: "c1", Options: []string{"theirs"}}); err != nil {
		t.Fatal(err)
	}
	if loaded, err := store.Load(t.Context(), "key"); err != nil || !slices.Equal(loaded.Options, []string{"theirs"}) {
		t.Fatalf("loaded %+v, %v", loaded, err)
	}
}
