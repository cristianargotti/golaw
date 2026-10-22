package cursor_test

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestCursorOversizedCapture(t *testing.T) {
	t.Parallel()
	for _, tool := range []string{"Write", "Delete"} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()
			curLargeCapture(t, tool)
		})
	}
}

func curLargeCapture(t *testing.T, tool string) {
	t.Helper()
	root := hooktest.Repository(t)
	data := curPatch(t, curBytes(t, "proof/testdata/write-capture/near/payload.json"), map[string]any{
		"tool_name": tool,
		"tool_input": map[string]any{
			"content": strings.Repeat("x", hook.MaxInputBytes+1),
			"nested":  map[string]any{"items": []any{map[string]any{"text": "PRIVATE_VALUE"}}},
			"path":    "PRIVATE_PATH",
		},
	})
	parsed, err := cursor.Oversized(bytes.NewReader(hooktest.Payload(data, root)))
	if err != nil {
		t.Fatal(err)
	}
	request, err := parsed.Decode(hook.Base{Root: root}, proof.NewTree(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := hook.Capture{
		Event: "preToolUse", Tool: tool,
		Keys: []string{"content", "nested", "nested.items", "nested.items.text", "path"},
	}
	curAssertLargeCapture(t, request, want)
	curEvaluateLargeCapture(t, root, parsed, want)
}

func curAssertLargeCapture(t *testing.T, request hook.Request, want hook.Capture) {
	t.Helper()
	if !request.Observe || request.Capture == nil ||
		!reflect.DeepEqual(*request.Capture, want) {
		t.Fatalf("capture = %+v", request)
	}
	if len(request.Content) != 0 || len(request.Event.Candidates) != 0 ||
		request.Event.Target != "" || request.Denial != "" {
		t.Fatalf("capture retained write values: %+v", request)
	}
}

func curEvaluateLargeCapture(t *testing.T, root string, parsed hook.Parsed, want hook.Capture) {
	t.Helper()
	calls := 0
	dependencies := hooktest.Dependencies(t, root, nil)
	dependencies.Capture = func(_ context.Context, _ hook.Repository, capture hook.Capture) error {
		calls++
		if !reflect.DeepEqual(capture, want) {
			t.Fatalf("stored capture = %+v", capture)
		}
		return nil
	}
	adapter := curAdapter()
	adapter.Parse = func([]byte) (hook.Parsed, error) { return parsed, nil }
	_, result := hook.Evaluate(t.Context(), "cursor", nil, adapter, dependencies)
	if result.Outcome != hook.Allow || calls != 1 {
		t.Fatalf("result = %+v, captures = %d", result, calls)
	}
}

func TestCursorOversizedShellCommand(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		command string
		bad     bool
	}{
		{"red command elided", strings.Repeat("x", hook.MaxInputBytes+1), true},
		{"near command retained", "git status", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := curPatch(t, curBytes(t, "proof/testdata/shell/near/payload.json"), map[string]any{
				"command": test.command,
				"future":  strings.Repeat("x", hook.MaxInputBytes+1),
			})
			request, err := curReducedRequest(t, data)
			curAssertLargeShell(t, request, err, test.bad)
		})
	}
}

func curAssertLargeShell(t *testing.T, request hook.Request, err error, bad bool) {
	t.Helper()
	if bad {
		if !errors.Is(err, model.ErrCannotJudge) {
			t.Fatalf("expected cannot judge, got %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if request.Event.Shell == nil || request.Event.Shell.Command != "git status" {
		t.Fatalf("retained shell = %+v", request)
	}
}
