package cursor_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestCursorShellObservation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		bad  bool
	}{
		{"red", true},
		{"near", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := curBytes(t, "proof/testdata/shell-observe/"+test.name+"/payload.json")
			request, err := curDecode(t, data)
			curCheckObservation(t, request, err, test.bad)
		})
	}
}

// curCheckObservation holds the decoded preToolUse Shell request to the
// plain shell request the core judges and keeps the baseline of: no
// Observe, no capture, the Shell tool, the envelope cwd and the tool use
// identity, with the red fixture carrying the command the shell family
// denies and the near fixture its control.
func curCheckObservation(t *testing.T, request hook.Request, err error, bad bool) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if request.Observe || request.Capture != nil || request.Event.Shell == nil ||
		request.Event.Shell.Tool != "Shell" || request.Event.ToolUseID == "" ||
		request.Event.Shell.Cwd != request.Event.Cwd {
		t.Fatalf("shell request = %+v", request)
	}
	if denied := strings.Contains(request.Event.Shell.Command, "--no-verify"); denied != bad {
		t.Fatalf("fixture command = %q, want red = %v", request.Event.Shell.Command, bad)
	}
}

func TestCursorShellPreJudgesTheCommandAndKeepsTheBaseline(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		command any
		want    hook.Outcome
		cannot  bool
	}{
		{"red the shell family denies", "git commit --no-verify -m x", hook.Deny, false},
		{"red a command that is no string", 7, hook.Deny, true},
		{"near the control", "git commit -m x", hook.Allow, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			root := hooktest.Repository(t)
			data := curBytes(t, "proof/testdata/shell-observe/near/payload.json")
			data = curPatch(t, data, map[string]any{
				"tool_input": map[string]any{
					"command":           test.command,
					"working_directory": "/ignored",
				},
			})
			dependencies := hooktest.Dependencies(t, root, map[string][]byte{
				"notes.md": []byte("before\n"),
			})
			event, result := hook.Evaluate(
				t.Context(), "cursor", hooktest.Payload(data, root), curAdapter(), dependencies,
			)
			if result.Outcome != test.want || result.CannotJudge != test.cannot {
				t.Fatalf("judgment = %+v, want %s", result, test.want)
			}
			if test.cannot {
				return
			}
			curCheckShellPreBaseline(t, root, event, dependencies.State, test.want == hook.Deny)
		})
	}
}

// curCheckShellPreBaseline holds the judged event to the envelope cwd and
// an allowed command to the baseline the core saved under its key; a
// denied command needs none.
func curCheckShellPreBaseline(t *testing.T, root string, event model.Event, state hook.Store, denied bool) {
	t.Helper()
	if event.Shell == nil || event.Shell.Cwd != root || event.Cwd != root {
		t.Fatalf("cwd = %+v", event)
	}
	if denied {
		return
	}
	key, err := hook.StateKey(root, event)
	if err != nil {
		t.Fatal(err)
	}
	before, err := state.Load(t.Context(), key)
	if err != nil || before.Files["notes.md"] != hook.Hash([]byte("before\n")) {
		t.Fatalf("baseline = %+v, error = %v", before, err)
	}
}

// The reason of a preToolUse Shell denial is the engine's, byte for byte
// the one beforeShellExecution gives for the same command (decision 13 of
// the plan, answered 2026-10-21: any denial wins, whichever event a Cursor
// build fires).
func TestCursorShellPreGivesTheReasonOfBeforeShellExecution(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	dependencies := hooktest.Dependencies(t, root, nil)
	reasons := map[string]string{}
	for event, path := range map[string]string{
		"preToolUse":           "proof/testdata/shell-observe/red/payload.json",
		"beforeShellExecution": "proof/testdata/shell/red/payload.json",
	} {
		_, result := hook.Evaluate(
			t.Context(), "cursor", hooktest.Payload(curBytes(t, path), root), curAdapter(), dependencies,
		)
		if result.Outcome != hook.Deny || result.CannotJudge || result.Reason == "" {
			t.Fatalf("%s: %+v", event, result)
		}
		reasons[event] = result.Reason
	}
	if reasons["preToolUse"] != reasons["beforeShellExecution"] {
		t.Fatalf("reasons differ: %q != %q", reasons["preToolUse"], reasons["beforeShellExecution"])
	}
}
