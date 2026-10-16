package hook_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// TestAnEventAsksForTheWorktreeOnlyWhenItsRulesRead keeps the listing of
// the worktree behind its first read: a Read event, whose rule judges
// names, never asks for it, so a listing that would fail changes nothing,
// while a write, whose candidate is bound to the base, asks, and a listing
// that fails there is cannot judge naming the snapshot.
func TestAnEventAsksForTheWorktreeOnlyWhenItsRulesRead(t *testing.T) {
	t.Parallel()
	read := request("safe")
	read.Event.Shape, read.Event.Tool, read.Event.Candidates, read.Content = model.ShapeRead, "Read", nil, nil
	read.Event.Read = &model.ReadRequest{Path: "a.go"}
	for _, tc := range []struct {
		name   string
		req    hook.Request
		judged bool
	}{
		{"read", read, false},
		{"write", request("safe"), true},
	} {
		adapter, dependencies := harness(t, tc.req)
		asked := 0
		dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
			asked++
			return nil, model.ErrUnreadable
		}
		_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
		failed := got.CannotJudge && strings.Contains(got.Reason, "snapshot")
		if (asked != 0) != tc.judged || failed != tc.judged || !tc.judged && got.Outcome != hook.Allow {
			t.Fatalf("%s asked for the worktree %d times: %+v", tc.name, asked, got)
		}
	}
}

// TestBeginStartsEveryEvent calls the binding's Begin once before each
// event, so what a binding learned for one event never answers the next.
func TestBeginStartsEveryEvent(t *testing.T) {
	t.Parallel()
	adapter, dependencies := harness(t, request("safe"))
	begun := 0
	dependencies.Begin = func() { begun++ }
	for range 2 {
		if _, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies); got.Outcome != hook.Allow {
			t.Fatal(got)
		}
	}
	if begun != 2 {
		t.Fatalf("Begin ran %d times for two events", begun)
	}
}

// TestAFailedListingAnswersAnEmptyUniverse reads the worktree a decoder
// receives after the listing failed, or answered no snapshot at all: every
// read answers nothing and the verdict is cannot judge naming the
// snapshot, so no rule judges an empty tree as clean.
func TestAFailedListingAnswersAnEmptyUniverse(t *testing.T) {
	t.Parallel()
	every := func(string) bool { return true }
	for _, failure := range []error{model.ErrUnreadable, nil} {
		adapter, dependencies := harness(t, request("safe"))
		dependencies.Snapshot = func(context.Context, hook.Repository) (model.Snapshot, error) {
			return nil, failure
		}
		empty := false
		adapter = decoding(adapter, func(snapshot model.Snapshot) {
			_, readErr := snapshot.Read("a.go")
			empty = readErr != nil && len(snapshot.Files()) == 0 && snapshot.Class("a.go") == model.ClassUnknown &&
				len(model.Select(snapshot, every)) == 0 && !model.Any(snapshot, every)
		})
		_, got := hook.Evaluate(t.Context(), "claude", []byte("{}"), adapter, dependencies)
		if !empty || !got.CannotJudge || !strings.Contains(got.Reason, "snapshot") {
			t.Fatalf("%v: empty %v, %+v", failure, empty, got)
		}
	}
}
