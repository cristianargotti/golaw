package hook_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// stampedTree is a large tree with the stat identity of its unread file.
type stampedTree struct {
	largeTree

	stamp string
}

// StatIdentity answers the stamp of big.sql and none for any other path.
func (s stampedTree) StatIdentity(path string) string {
	if path == "big.sql" {
		return s.stamp
	}
	return ""
}

// TestIdentitiesKeepTheStatOfALargeFile distinguishes a stat stamp from a
// size alone, so an unchanged stamp leaves no change and another does.
func TestIdentitiesKeepTheStatOfALargeFile(t *testing.T) {
	t.Parallel()
	tree := stampedTree{largeTree{size: model.MaxReadBytes + 1}, "before"}
	before, err := hook.Identities(t.Context(), tree, []string{"big.sql"})
	if err != nil || before["big.sql"] != "large-stat:before" {
		t.Fatalf("%v, %v", before, err)
	}
	for _, stamp := range []string{"before", "after", ""} {
		tree.stamp = stamp
		after, identityErr := hook.Identities(t.Context(), tree, []string{"big.sql"})
		if identityErr != nil || (len(hook.Changed(before, after)) != 0) != (stamp != "before") {
			t.Fatalf("%s: %v, %v", stamp, after, identityErr)
		}
		if stamp == "" && !strings.HasPrefix(after["big.sql"], "large:") {
			t.Fatal(after)
		}
	}
}

// TestALargeFileStatNamesOnlyAChange drives post events over the same
// dirty path, so only a changed stat leaves the notice of its unread bytes.
func TestALargeFileStatNamesOnlyAChange(t *testing.T) {
	t.Parallel()
	for _, stamp := range []string{"before", "after"} {
		req := shellRequest()
		adapter, dependencies := harness(t, req)
		tree := stampedTree{largeTree{size: model.MaxReadBytes + 1}, "before"}
		dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) { return tree, nil }
		dependencies.Changes = func(context.Context, hook.Repository, []string) (hook.Changes, error) {
			return hook.Changes{Snapshot: tree, Paths: []string{"big.sql"}}, nil
		}
		event, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
		if got.Outcome != hook.Allow || got.Notice != "" {
			t.Fatal(got)
		}
		tree.stamp = stamp
		_, got = hook.Evaluate(t.Context(), "claude", []byte("{}"), asPostEvent(&req, event.Cwd, false), dependencies)
		changed := stamp != "before"
		if got.Outcome != hook.Allow || got.CannotJudge || strings.Contains(got.Notice, "big.sql") != changed ||
			strings.Contains(got.Human, "big.sql") != changed || !changed && (got.Notice != "" || got.Human != "") {
			t.Fatalf("%s: %+v", stamp, got)
		}
	}
}

// TestAWriteBindsTheLargeBaseStat drives freshness through the deferred
// snapshot, so a changed stat cannot answer for the base the decoder saw.
func TestAWriteBindsTheLargeBaseStat(t *testing.T) {
	t.Parallel()
	for _, stamp := range []string{"before", "after"} {
		req := request("safe")
		req.Event.Candidates[0].Path, req.Event.Candidates[0].BaseHash = "big.sql", ""
		req.Content = map[string][]byte{"big.sql": []byte("safe")}
		adapter, dependencies := harness(t, req)
		calls := 0
		dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
			calls++
			identity := "before"
			if calls > 1 {
				identity = stamp
			}
			return stampedTree{largeTree{size: model.MaxReadBytes + 1}, identity}, nil
		}
		_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
		changed := stamp != "before"
		if calls != 2 || got.CannotJudge != changed || strings.Contains(got.Reason, "candidate base changed") != changed ||
			!changed && got.Outcome != hook.Allow {
			t.Fatalf("%s, %d snapshots: %+v", stamp, calls, got)
		}
	}
}
