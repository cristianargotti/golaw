package cursor_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestCursorEnvelope(t *testing.T) {
	t.Parallel()
	near := curBytes(t, "proof/testdata/envelope/near/payload.json")
	tests := []struct {
		name string
		data []byte
		bad  bool
	}{
		{"red foreign", curBytes(t, "proof/testdata/envelope/red/payload.json"), true},
		{"red null", []byte("null"), true},
		{"red array", []byte("[]"), true},
		{"red missing identity", []byte(`{"hook_event_name":"beforeShellExecution"}`), true},
		{"red foreign marker", curPatch(t, near, map[string]any{"turn_id": nil}), true},
		{"red roots type", curPatch(t, near, map[string]any{"workspace_roots": 1}), true},
		{"near documented", near, false},
		{"near future metadata", curPatch(t, near, map[string]any{"future": true}), false},
		{"near empty roots", curPatch(t, near, map[string]any{"workspace_roots": []string{}}), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			curCheckEnvelope(t, test.data, test.bad)
		})
	}
}

func curCheckEnvelope(t *testing.T, data []byte, bad bool) {
	t.Helper()
	parsed, err := cursor.Parse(data)
	if bad {
		if !errors.Is(err, model.ErrCannotJudge) {
			t.Fatalf("expected cannot judge, got %v", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("accepted payload: %v", err)
	}
	if parsed.Decode == nil || parsed.Event.Agent != "cursor" {
		t.Fatalf("incomplete parsed event: %+v", parsed.Event)
	}
}

func TestCursorHeaderIdentity(t *testing.T) {
	t.Parallel()
	data := curBytes(t, "proof/testdata/envelope/near/payload.json")
	data = curPatch(t, data, map[string]any{
		"hook_event_name": "preToolUse",
		"tool_name":       "Shell",
		"tool_use_id":     "tool",
		"tool_input":      map[string]any{"command": "git status"},
	})
	parsed, err := cursor.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	event := parsed.Event
	if event.SessionID != "conversation" || event.AgentID != "generation" ||
		event.ToolUseID != "tool" || event.Cwd != hooktest.Root {
		t.Fatalf("identity = %+v", event)
	}
	first, err := hook.StateKey("repository", event)
	if err != nil {
		t.Fatal(err)
	}
	event.Shape = model.ShapePost
	second, err := hook.StateKey("repository", event)
	if err != nil || first != second {
		t.Fatalf("shape changed the key: %q %q %v", first, second, err)
	}
}

func TestCursorInitialShellRoute(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		want hook.Outcome
	}{
		{"red", hook.Deny},
		{"near", hook.Allow},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := curBytes(t, "proof/testdata/shell/"+test.name+"/payload.json")
			_, result := hooktest.Evaluate(t, "cursor", curAdapter(), data, nil)
			if result.Outcome != test.want {
				t.Fatalf("outcome = %s, want %s: %s", result.Outcome, test.want, result.Reason)
			}
		})
	}
}
