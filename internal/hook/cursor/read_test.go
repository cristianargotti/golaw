package cursor_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestCursorReadNamesAndParity(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		path string
		want hook.Outcome
	}{
		{"red", ".env", hook.Deny},
		{"near", "main.go", hook.Allow},
		{"near credential source", "credentials.go", hook.Allow},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := curBytes(t, "proof/testdata/read/near/payload.json")
			data = curPatch(t, data, map[string]any{
				"file_path": "/project/" + test.path,
				"content":   "PRIVATE_VALUE",
			})
			curCheckReadRequest(t, data, test.path)
			curCheckReadParity(t, data, test.path, test.want)
		})
	}
}

func curCheckReadRequest(t *testing.T, data []byte, path string) {
	t.Helper()
	request, err := curDecode(t, data)
	if err != nil {
		t.Fatal(err)
	}
	if request.Event.Shape != model.ShapeRead || request.Event.Read == nil ||
		request.Event.Read.Path != path || len(request.Content) != 0 {
		t.Fatalf("read = %+v", request)
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "PRIVATE_VALUE") {
		t.Fatal("read content reached the normalized request")
	}
}

func curCheckReadParity(t *testing.T, data []byte, path string, want hook.Outcome) {
	t.Helper()
	_, result := hooktest.Evaluate(t, "cursor", curAdapter(), data, nil)
	other := []byte(`{"hook_event_name":"PreToolUse","tool_name":"Read",` +
		`"cwd":"/project","session_id":"session","tool_use_id":"tool",` +
		`"tool_input":{"file_path":"/project/` + path + `"}}`)
	_, expected := hooktest.Evaluate(t, "claude", hook.Adapter{
		Parse: claude.Parse, Encode: claude.Encode,
	}, other, nil)
	if result.Outcome != want || result.Reason != expected.Reason {
		t.Fatalf("read parity: %+v, expected %+v", result, expected)
	}
}
