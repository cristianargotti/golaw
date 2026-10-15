package hook_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestIdentitiesDetectContentChanges(t *testing.T) {
	t.Parallel()
	before, err := hook.Identities(t.Context(), proof.NewTree(map[string][]byte{
		"dirty.go": []byte("old"), "gone.go": {}, "same.go": []byte("same"),
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	after, err := hook.Identities(t.Context(), proof.NewTree(map[string][]byte{
		"dirty.go": []byte("new"), "new.go": {}, "same.go": []byte("same"),
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(hook.Changed(before, after),
		[]string{"dirty.go", "gone.go", "new.go"}) {
		t.Fatal(hook.Changed(before, after))
	}
	absent, err := hook.Identities(t.Context(), proof.NewTree(nil), []string{"empty.go"})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := hook.Identities(t.Context(),
		proof.NewTree(map[string][]byte{"empty.go": {}}), []string{"empty.go"})
	if err != nil {
		t.Fatal(err)
	}
	if maps.Equal(absent, empty) {
		t.Fatal("absence equals empty content")
	}
	if hook.MapHash(before) != hook.MapHash(maps.Clone(before)) {
		t.Fatal("identity depends on map iteration")
	}
}

func TestStateKeyIsolation(t *testing.T) {
	t.Parallel()
	base := model.Event{
		Agent: "claude", SessionID: "session",
		AgentID: "agent", ToolUseID: "tool",
	}
	original, err := hook.StateKey("repository", base)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		event, repository := base, "repository"
		switch i {
		case 0:
			repository = "other"
		case 1:
			event.SessionID = "other"
		case 2:
			event.AgentID = "other"
		case 3:
			event.ToolUseID = "other"
		case 4:
			event.Agent = "codex"
		}
		key, err := hook.StateKey(repository, event)
		if err != nil || key == original {
			t.Fatalf("identity collision: %d, %v", i, err)
		}
	}
	base.ToolUseID = ""
	if _, err := hook.StateKey("repository", base); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatal(err)
	}
}

func TestIdentityBoundsAndCancellation(t *testing.T) {
	t.Parallel()
	tree := proof.NewTree(map[string][]byte{
		"large": make([]byte, model.MaxReadBytes+1),
	})
	if _, err := hook.Identities(t.Context(), tree, nil); err == nil {
		t.Fatal("oversized content accepted")
	}
	if _, err := hook.Identities(t.Context(), proof.NewTree(nil),
		[]string{"../outside"}); err == nil {
		t.Fatal("escaping path accepted")
	}
}

func TestIdentityPathLimitAndCancelledCapture(t *testing.T) {
	t.Parallel()
	paths := make([]string, 65536)
	for i := range paths {
		paths[i] = fmt.Sprintf("file-%d.go", i)
	}
	if got, err := hook.Identities(t.Context(), proof.NewTree(nil), paths); err != nil || len(got) != len(paths) {
		t.Fatalf("valid repository capture failed: %d, %v", len(got), err)
	}
	over := make([]string, 0, len(paths)+1)
	over = append(over, paths...)
	over = append(over, "overflow.go")
	if _, err := hook.Identities(t.Context(), proof.NewTree(nil), over); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := hook.Identities(ctx, proof.NewTree(nil), []string{"a.go"}); !errors.Is(err, model.ErrDeadline) {
		t.Fatal(err)
	}
}
