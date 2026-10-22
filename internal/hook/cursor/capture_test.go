package cursor_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

var errCaptureStore = errors.New("capture storage unavailable")

func TestCursorCaptureRoutes(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"write-capture", "delete-capture"} {
		for _, test := range []struct {
			name string
			bad  bool
		}{
			{"red", true},
			{"near", false},
		} {
			t.Run(route+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				path := "proof/testdata/" + route + "/" + test.name + "/payload.json"
				request, err := curDecode(t, curBytes(t, path))
				curCheckCaptureRequest(t, request, err, test.bad)
			})
		}
	}
}

func curCheckCaptureRequest(t *testing.T, request hook.Request, err error, bad bool) {
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
	if !request.Observe || request.Capture == nil ||
		request.Event.Shape != model.ShapeWrite ||
		request.Event.Target != "" || len(request.Event.Candidates) != 0 ||
		len(request.Content) != 0 || request.Denial != "" {
		t.Fatalf("write enforcement escaped preview: %+v", request)
	}
}

func TestCursorCaptureContainsKeysOnly(t *testing.T) {
	t.Parallel()
	data := curBytes(t, "proof/testdata/write-capture/near/payload.json")
	data = curPatch(t, data, map[string]any{
		"tool_input": map[string]any{
			"path":   "VALUE_A",
			"nested": map[string]any{"content": "VALUE_B"},
			"items":  []any{map[string]any{"text": "VALUE_C"}},
		},
	})
	request, err := curDecode(t, data)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"items", "items.text", "nested", "nested.content", "path"}
	if !reflect.DeepEqual(request.Capture.Keys, want) {
		t.Fatalf("keys = %q", request.Capture.Keys)
	}
	encoded, err := json.Marshal(request.Capture)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"VALUE_A", "VALUE_B", "VALUE_C"} {
		if strings.Contains(string(encoded), value) {
			t.Fatal("capture retained a value")
		}
	}
}

// TestCursorCaptureNamesTheTypeOfAnInputThatIsNoObject captures a Write
// whose tool_input is no object by its JSON type and allows it: the
// payload is undocumented and Cursor already stringifies other fields, so
// a shape the decoder does not know is the A4 exception, never a denial of
// every Cursor write (added by the review of the plan).
func TestCursorCaptureNamesTheTypeOfAnInputThatIsNoObject(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input any
		key   string
	}{
		{[]any{}, "<array>"},
		{nil, "<null>"},
		{"VALUE_A", "<string>"},
		{float64(1), "<number>"},
		{true, "<boolean>"},
	} {
		data := curPatch(t, curBytes(t, "proof/testdata/write-capture/near/payload.json"), map[string]any{"tool_input": test.input})
		request, err := curDecode(t, data)
		if err != nil || !request.Observe || request.Capture == nil || !reflect.DeepEqual(request.Capture.Keys, []string{test.key}) {
			t.Errorf("%v: request = %+v, %v", test.input, request, err)
		}
	}
}

func TestCursorCaptureFailureStillAllowsAndSavesState(t *testing.T) {
	t.Parallel()
	for _, captureError := range []error{nil, errCaptureStore} {
		t.Run("capture", func(t *testing.T) {
			t.Parallel()
			root := hooktest.Repository(t)
			dependencies := hooktest.Dependencies(t, root, map[string][]byte{
				"notes.md": []byte("before\n"),
			})
			calls := 0
			dependencies.Capture = func(
				_ context.Context, repository hook.Repository, capture hook.Capture,
			) error {
				calls++
				curCheckCaptureCall(t, root, repository, capture)
				return captureError
			}
			data := curBytes(t, "proof/testdata/write-capture/near/payload.json")
			event, result := hook.Evaluate(
				t.Context(), "cursor", hooktest.Payload(data, root), curAdapter(), dependencies,
			)
			curCheckCaptureResult(t, calls, result)
			curCheckCaptureBaseline(t, root, event, dependencies.State)
		})
	}
}

func curCheckCaptureCall(
	t *testing.T, root string, repository hook.Repository, capture hook.Capture,
) {
	t.Helper()
	if repository.Root != root || capture.Event != "preToolUse" ||
		capture.Tool != "Write" {
		t.Fatalf("capture = %+v, repository = %+v", capture, repository)
	}
}

func curCheckCaptureResult(t *testing.T, calls int, result hook.Result) {
	t.Helper()
	if calls != 1 || result.Outcome != hook.Allow {
		t.Fatalf("calls = %d, result = %+v", calls, result)
	}
}

func curCheckCaptureBaseline(t *testing.T, root string, event model.Event, state hook.Store) {
	t.Helper()
	key, err := hook.StateKey(root, event)
	if err != nil {
		t.Fatal(err)
	}
	before, err := state.Load(t.Context(), key)
	if err != nil || before.Files["notes.md"] != hook.Hash([]byte("before\n")) {
		t.Fatalf("baseline = %+v, error = %v", before, err)
	}
}
