package hook

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// postRun is the execution of the post event of a shell command whose pre
// event left a baseline in a store of its own, with the dependencies that
// read that store.
func postRun(t *testing.T, before Baseline, head string) (execution, Dependencies) {
	t.Helper()
	event := model.Event{Agent: "claude", SessionID: "session", ToolUseID: "shell", Tool: "Bash", Shape: model.ShapePost}
	store := Store{Dir: t.TempDir(), Now: time.Now()}
	key, err := StateKey("repo", event)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(t.Context(), key, "input", before); err != nil {
		t.Fatal(err)
	}
	run := execution{repository: Repository{Identity: "repo", Head: head}, request: Request{Event: event}}
	return run, Dependencies{State: store}
}

// TestChangedPathsNamesAMoveGitCannotRead answers a move of HEAD the binding
// cannot read with the gap of the changes of the command, beside the gap of
// each path whose status alone changed, while a path whose bytes changed
// since the pre event named it is still judged.
func TestChangedPathsNamesAMoveGitCannotRead(t *testing.T) {
	t.Parallel()
	run, deps := postRun(t, Baseline{Files: map[string]string{"dirty.go": Hash([]byte("one"))}, Head: "c1"}, "c2")
	snapshot := proof.NewTree(map[string][]byte{"dirty.go": []byte("two"), "mode.go": []byte("hello\n")})
	deps.Changes = func(context.Context, Repository, []string) (Changes, error) {
		return Changes{Snapshot: snapshot, Paths: []string{"dirty.go", "mode.go"}, Head: "c2", Blobs: map[string]string{"mode.go": helloSHA1}}, nil
	}
	deps.Moved = func(context.Context, Repository, string, string) (map[string]string, error) {
		return nil, model.ErrUnreadable
	}
	changed, gaps, err := run.changedPaths(t.Context(), deps)
	want := []Gap{{Cause: causeMove}, {Path: "mode.go", Cause: causeMove}}
	if err != nil || !slices.Equal(changed, []string{"dirty.go"}) || !slices.Equal(gaps, want) {
		t.Fatalf("changed %v, gaps %+v, %v", changed, gaps, err)
	}
}
