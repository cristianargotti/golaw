package cursor_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

// TestCursorRoutesRefuseWhatTheyCannotRead holds every route to cannot
// judge when the documented field it reads is absent or of another type,
// and an event or a tool no route judges to the same answer, so the
// decoder never invents a verdict the policy did not give.
func TestCursorRoutesRefuseWhatTheyCannotRead(t *testing.T) {
	t.Parallel()
	shell := curBytes(t, "proof/testdata/shell/near/payload.json")
	pre := curBytes(t, "proof/testdata/shell-observe/near/payload.json")
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"shell without command", []byte(`{"hook_event_name":"beforeShellExecution","conversation_id":"conversation","workspace_roots":["/project"],"cwd":"/project"}`)},
		{"pre shell without input", []byte(`{"hook_event_name":"preToolUse","conversation_id":"conversation","workspace_roots":["/project"],"cwd":"/project","tool_name":"Shell","tool_use_id":"tool"}`)},
		{"pre shell input array", curPatch(t, pre, map[string]any{"tool_input": []any{}})},
		{"pre shell input null", curPatch(t, pre, map[string]any{"tool_input": nil})},
		{"pre tool no route judges", curPatch(t, pre, map[string]any{"tool_name": "Grep"})},
		{"pre tool name no string", curPatch(t, pre, map[string]any{"tool_name": 7})},
		{"event no route judges", curPatch(t, shell, map[string]any{"hook_event_name": "afterFileEdit"})},
		{"roots holding no string", curPatch(t, shell, map[string]any{"workspace_roots": []any{1}})},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := curDecode(t, test.data); !errors.Is(err, model.ErrCannotJudge) {
				t.Fatalf("expected cannot judge, got %v", err)
			}
		})
	}
}

// TestCursorHeaderKeepsTheShapeOfEveryDocumentedEvent pins the header a
// read or post event carries before its route decodes it, so the answer
// of a later failure keeps the documented form of that event, and the
// working directory of an event without cwd taken from the first root.
func TestCursorHeaderKeepsTheShapeOfEveryDocumentedEvent(t *testing.T) {
	t.Parallel()
	shell := curBytes(t, "proof/testdata/shell-observe/near/payload.json")
	read := []byte(`{"hook_event_name":"beforeReadFile","conversation_id":"conversation","generation_id":"generation","workspace_roots":["/project","/other"],"file_path":"/project/notes.md"}`)
	for _, test := range []struct {
		name   string
		data   []byte
		shape  model.EventShape
		tool   string
		failed bool
	}{
		{"read", read, model.ShapeRead, "Read", false},
		{"post", curPatch(t, shell, map[string]any{"hook_event_name": "postToolUse"}), model.ShapePost, "Shell", false},
		{"post failure", curPatch(t, shell, map[string]any{"hook_event_name": "postToolUseFailure", "tool_name": "Write"}), model.ShapePost, "Write", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := cursor.Parse(test.data)
			if err != nil {
				t.Fatal(err)
			}
			event := parsed.Event
			if event.Shape != test.shape || event.Tool != test.tool || event.Cwd != hooktest.Root {
				t.Fatalf("header = %+v", event)
			}
			if (event.Post != nil && event.Post.Failed) != test.failed || (test.shape == model.ShapePost) != (event.Post != nil) {
				t.Fatalf("post = %+v", event.Post)
			}
		})
	}
}

// TestCursorWriteWithoutInputIsCapturedByItsAbsence keeps a Write whose
// envelope carries no tool_input on the capture path of A4: the payload is
// undocumented, so its absence is recorded and the write allowed.
func TestCursorWriteWithoutInputIsCapturedByItsAbsence(t *testing.T) {
	t.Parallel()
	data := []byte(`{"hook_event_name":"preToolUse","conversation_id":"conversation","generation_id":"generation","workspace_roots":["/project"],"cwd":"/project","tool_name":"Write","tool_use_id":"tool"}`)
	request, err := curDecode(t, data)
	if err != nil || !request.Observe || request.Capture == nil ||
		!reflect.DeepEqual(request.Capture.Keys, []string{"<absent>"}) {
		t.Fatalf("request = %+v, %v", request, err)
	}
}

// TestCursorDenialLeavesAsThePermissionJSON holds a pre denial and the
// refused envelope of another agent to the permission JSON with exit 0,
// which Cursor reads under failClosed, the agent reading the English
// reason and the person the human line.
func TestCursorDenialLeavesAsThePermissionJSON(t *testing.T) {
	t.Parallel()
	event := model.Event{Agent: "cursor", Shape: model.ShapeShell, Tool: "Shell"}
	result := hook.Result{Outcome: hook.Deny, Reason: "reason", Human: "linha humana"}
	answer := curPermission(t, cursor.Encode(event, result))
	if answer["permission"] != "deny" || answer["agent_message"] != result.Reason ||
		answer["user_message"] != result.Human {
		t.Fatalf("answer = %+v", answer)
	}
	reply := hooktest.Answer(t, "cursor", curAdapter(), []byte(`{"hook_event_name":"PreToolUse"}`), nil)
	if refused := curPermission(t, reply); refused["permission"] != "deny" ||
		!strings.Contains(refused["agent_message"], "cannot judge") {
		t.Fatalf("refused = %+v", refused)
	}
}

func curPermission(t *testing.T, reply hook.Reply) map[string]string {
	t.Helper()
	var answer map[string]string
	if err := json.Unmarshal(reply.Stdout, &answer); err != nil || reply.ExitCode != 0 {
		t.Fatalf("reply = %+v, %v", reply, err)
	}
	return answer
}

// TestCursorOversizedInputCannotBeJudgedWhenIncomplete holds an input over
// the bound that is no complete object to cannot judge.
func TestCursorOversizedInputCannotBeJudgedWhenIncomplete(t *testing.T) {
	t.Parallel()
	if _, err := cursor.Oversized(strings.NewReader("{")); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("expected cannot judge, got %v", err)
	}
}
