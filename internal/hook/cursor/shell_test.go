package cursor_test

import (
	"errors"
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestCursorDedicatedShellHasNoBaselineIdentity(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	data := curBytes(t, "proof/testdata/shell/near/payload.json")
	data = curPatch(t, data, map[string]any{
		"tool_use_id": "unrelated-metadata",
		"tool_name":   "unrelated-metadata",
	})
	dependencies := hooktest.Dependencies(t, root, nil)
	event, result := hook.Evaluate(
		t.Context(), "cursor", hooktest.Payload(data, root), curAdapter(), dependencies,
	)
	if result.Outcome != hook.Allow || event.Tool != "Shell" {
		t.Fatalf("result = %+v, event = %+v", result, event)
	}
	if _, err := hook.StateKey(root, event); err == nil {
		t.Fatal("dedicated shell event created a state key")
	}
	entries, err := os.ReadDir(dependencies.State.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("dedicated shell event saved %d state files", len(entries))
	}
}

func TestCursorShellPolicyParity(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		command string
		want    hook.Outcome
	}{
		{"red bypass", "git commit --no-verify -m x", hook.Deny},
		{"near commit", "git commit -m x", hook.Allow},
		{"red credential printer", "printenv", hook.Deny},
		{"near command name", "printf hello", hook.Allow},
		{"red wrapper", "bash -lc 'git commit --no-verify -m x'", hook.Deny},
		{"near wrapper", "bash -lc 'git status'", hook.Allow},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := curBytes(t, "proof/testdata/shell/near/payload.json")
			data = curPatch(t, data, map[string]any{"command": test.command})
			_, actual := hooktest.Evaluate(t, "cursor", curAdapter(), data, nil)
			other := []byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash",` +
				`"cwd":"/project","session_id":"session","tool_use_id":"tool",` +
				`"tool_input":{}}`)
			other = curPatch(t, other, map[string]any{
				"tool_input": map[string]any{"command": test.command},
			})
			_, expected := hooktest.Evaluate(t, "claude", hook.Adapter{
				Parse: claude.Parse, Encode: claude.Encode,
			}, other, nil)
			if actual.Outcome != test.want || actual.Reason != expected.Reason {
				t.Fatalf("shell parity: %+v, expected %+v", actual, expected)
			}
		})
	}
}

func TestCursorShellCannotJudgeWrongCommandType(t *testing.T) {
	t.Parallel()
	base := curBytes(t, "proof/testdata/shell/near/payload.json")
	for _, test := range []struct {
		name    string
		command any
		bad     bool
	}{
		{"red null", nil, true},
		{"red number", 9, true},
		{"near string", "git status", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := curPatch(t, base, map[string]any{"command": test.command})
			_, err := curDecode(t, data)
			if test.bad && !errors.Is(err, model.ErrCannotJudge) {
				t.Fatalf("expected cannot judge, got %v", err)
			}
			if !test.bad && err != nil {
				t.Fatal(err)
			}
		})
	}
}
