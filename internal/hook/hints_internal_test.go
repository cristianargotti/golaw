package hook

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// hintedRun is the post event of a shell command the host reports a diff
// for, the files of that diff as its hints, over a store saved at a known
// moment that holds the baseline of its pre event when one is given.
func hintedRun(t *testing.T, before *Baseline, at time.Time, hints ...string) (execution, Dependencies) {
	t.Helper()
	event := model.Event{Agent: "claude", SessionID: "session", ToolUseID: "shell", Tool: "Bash", Shape: model.ShapePost}
	store := Store{Dir: t.TempDir(), Now: at}
	if before != nil {
		key, err := StateKey("repo", event)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Save(t.Context(), key, "input", *before); err != nil {
			t.Fatal(err)
		}
	}
	run := execution{repository: Repository{Identity: "repo", Head: "c2"}, request: Request{Event: event, Hints: hints}}
	return run, Dependencies{State: store}
}

// hostDiffDeps answer a command that moved HEAD from c1 to c2 over a commit
// of its own adding mine.md and a teammate's of an hour before adding
// theirs.md, after which git status names mode.md, whose mode alone
// changed, and notes.md, whose carriage returns git cannot convert to
// compare them with the blob it stores.
func hostDiffDeps(deps Dependencies, at time.Time) Dependencies {
	snapshot := proof.NewTree(map[string][]byte{
		"mine.md": []byte("bad"), "theirs.md": []byte("bad"), "mode.md": []byte("hello\n"), "notes.md": []byte("hello\r\n"),
	})
	deps.Changes = func(context.Context, Repository, []string) (Changes, error) {
		return Changes{
			Snapshot: snapshot, Paths: []string{"mode.md", "notes.md"}, Head: "c2",
			Blobs: map[string]string{"mode.md": helloSHA1, "notes.md": helloSHA1},
		}, nil
	}
	deps.Moved = func(context.Context, Repository, string, string) (map[string]string, error) {
		return map[string]string{"mine.md": "", "theirs.md": ""}, nil
	}
	deps.Committer = func(context.Context, Repository) (string, error) { return "me <me@example.com>", nil }
	deps.Commits = func(context.Context, Repository, string, string, int64) ([]Commit, error) {
		return []Commit{
			{Committer: "me <me@example.com>", Time: at.Unix(), Paths: []string{"mine.md"}},
			{Committer: "mate <mate@example.com>", Time: at.Unix() - 3600, Paths: []string{"theirs.md"}},
		}, nil
	}
	deps.Hashes = func(context.Context, Repository, []string) (map[string]string, error) {
		return nil, model.ErrUnreadable
	}
	return deps
}

// TestPostPathsJudgeTheHostDiffAsDetectionClassifiesIt holds the files the
// host reports a shell command changed to what change detection answers
// for them without that report: a file a teammate's commit brought is named
// as arrived, one whose mode alone changed is left alone, one git cannot
// convert is a gap and one git status names at neither end is unchanged, so
// only the file the command committed is judged; without a baseline that
// classifies them, the files the host reports are judged, with the notice
// of the missing baseline.
func TestPostPathsJudgeTheHostDiffAsDetectionClassifiesIt(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.October, 17, 12, 0, 0, 0, time.UTC)
	hints := []string{"clean.md", "mine.md", "mode.md", "notes.md", "theirs.md"}
	run, deps := hintedRun(t, &Baseline{Files: map[string]string{}, Head: "c1"}, at, hints...)
	paths, gaps := run.postPaths(t.Context(), hostDiffDeps(deps, at))
	want := append(pathGaps([]string{"notes.md"}, causeConvert), pathGaps([]string{"theirs.md"}, causeArrived)...)
	if !slices.Equal(paths, []string{"mine.md"}) || !slices.Equal(gaps, want) {
		t.Fatalf("judged %v, gaps %+v", paths, gaps)
	}
	run, deps = hintedRun(t, nil, at, hints...)
	paths, gaps = run.postPaths(t.Context(), hostDiffDeps(deps, at))
	if !slices.Equal(paths, hints) || !slices.Equal(gaps, []Gap{{Cause: causeBaseline}}) {
		t.Fatalf("without a baseline: judged %v, gaps %+v", paths, gaps)
	}
}
