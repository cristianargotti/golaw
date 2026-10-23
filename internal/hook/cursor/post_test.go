package cursor_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func curPostPair(t *testing.T, route, variant, text string) (model.Event, model.Event, hook.Result) {
	t.Helper()
	root := hooktest.Repository(t)
	before := hooktest.Dependencies(t, root, map[string][]byte{
		"notes.md": []byte("before\n"),
	})
	after := hooktest.Dependencies(t, root, map[string][]byte{
		"notes.md": []byte(text),
	})
	after.State = before.State
	dir := "proof/testdata/" + route + "/" + variant + "/"
	pre := hooktest.Payload(curBytes(t, dir+"pre.json"), root)
	post := hooktest.Payload(curBytes(t, dir+"payload.json"), root)
	first, initial := hook.Evaluate(t.Context(), "cursor", pre, curAdapter(), before)
	if initial.Outcome != hook.Allow {
		t.Fatalf("pre event = %+v", initial)
	}
	last, result := hook.Evaluate(t.Context(), "cursor", post, curAdapter(), after)
	firstKey, err := hook.StateKey(root, first)
	if err != nil {
		t.Fatal(err)
	}
	lastKey, err := hook.StateKey(root, last)
	if err != nil || firstKey != lastKey {
		t.Fatalf("pre/post key mismatch: %q %q %v", firstKey, lastKey, err)
	}
	return first, last, result
}

func TestCursorPostRoutes(t *testing.T) {
	t.Parallel()
	for _, route := range []string{
		"post-shell", "post-shell-failure", "post-write", "post-write-failure",
	} {
		for _, test := range []struct {
			name string
			text string
			want hook.Outcome
		}{
			{"red", "after " + string(rune(0x2014)) + " change\n", hook.Feedback},
			{"near", "after - change\n", hook.Allow},
		} {
			t.Run(route+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				_, event, result := curPostPair(t, route, test.name, test.text)
				if result.Outcome != test.want || event.Post == nil {
					t.Fatalf("post = %+v, result = %+v", event, result)
				}
			})
		}
	}
}

func TestCursorPostMissingBaselineIsAdvisory(t *testing.T) {
	t.Parallel()
	data := curBytes(t, "proof/testdata/post-shell/red/payload.json")
	_, result := hooktest.Evaluate(t, "cursor", curAdapter(), data, nil)
	if result.Outcome != hook.Allow || result.Notice == "" {
		t.Fatalf("missing baseline = %+v", result)
	}
}

func TestCursorDeniedCommandPostHasNoFinding(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	dependencies := hooktest.Dependencies(t, root, map[string][]byte{
		"notes.md": []byte("before\n"),
	})
	pre := curBytes(t, "proof/testdata/post-shell-failure/near/pre.json")
	post := curBytes(t, "proof/testdata/post-shell-failure/near/payload.json")
	post = curPatch(t, post, map[string]any{"failure_type": "permission_denied"})
	_, before := hook.Evaluate(
		t.Context(), "cursor", hooktest.Payload(pre, root), curAdapter(), dependencies,
	)
	_, after := hook.Evaluate(
		t.Context(), "cursor", hooktest.Payload(post, root), curAdapter(), dependencies,
	)
	if before.Outcome != hook.Allow || after.Outcome != hook.Allow || after.Notice != "" {
		t.Fatalf("unchanged denied command: %+v, %+v", before, after)
	}
}

// TestCursorPostAfterADenialAnswersNothing follows the real flow of D7 and
// decision 13: golaw denies the preToolUse Shell, which keeps no baseline,
// and Cursor then sends postToolUseFailure with permission_denied for a
// command that never ran; the answer is {} with no advisory about it.
func TestCursorPostAfterADenialAnswersNothing(t *testing.T) {
	t.Parallel()
	root := hooktest.Repository(t)
	dependencies := hooktest.Dependencies(t, root, map[string][]byte{"notes.md": []byte("before\n")})
	var red struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(curBytes(t, "proof/testdata/shell/red/payload.json"), &red); err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"command": red.Command, "working_directory": "/project"}
	pre := curPatch(t, curBytes(t, "proof/testdata/post-shell-failure/near/pre.json"), map[string]any{"tool_input": input})
	post := curPatch(t, curBytes(t, "proof/testdata/post-shell-failure/near/payload.json"), map[string]any{
		"tool_input": input, "failure_type": "permission_denied", "error_message": "denied by a hook",
	})
	_, before := hook.Evaluate(t.Context(), "cursor", hooktest.Payload(pre, root), curAdapter(), dependencies)
	event, after := hook.Evaluate(t.Context(), "cursor", hooktest.Payload(post, root), curAdapter(), dependencies)
	if before.Outcome != hook.Deny || after.Outcome != hook.Allow || after.Notice != "" || after.Human != "" {
		t.Fatalf("pre = %+v, post = %+v", before, after)
	}
	if reply := curAdapter().Encode(event, after); reply.ExitCode != 0 || strings.TrimSpace(string(reply.Stdout)) != "{}" {
		t.Fatalf("reply = %+v", reply)
	}
}

// TestCursorPostRefusesWhatItCannotRead holds a post event to cannot judge
// when its tool is one no post route reads or its documented result string
// is null or of another type, so the decoder never answers a verdict the
// policy did not give.
func TestCursorPostRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()
	post := curBytes(t, "proof/testdata/post-shell/near/payload.json")
	failure := curBytes(t, "proof/testdata/post-shell-failure/near/payload.json")
	for _, test := range []struct {
		name string
		data []byte
	}{
		{"post tool no route reads", curPatch(t, post, map[string]any{"tool_name": "Delete"})},
		{"failure tool no route reads", curPatch(t, failure, map[string]any{"tool_name": "MCP:Shell"})},
		{"post output null", curPatch(t, post, map[string]any{"tool_output": nil})},
		{"post output no string", curPatch(t, post, map[string]any{"tool_output": map[string]any{"exitCode": 0}})},
		{"failure message null", curPatch(t, failure, map[string]any{"error_message": nil})},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := curDecode(t, test.data); !errors.Is(err, model.ErrCannotJudge) {
				t.Fatalf("expected cannot judge, got %v", err)
			}
		})
	}
}
