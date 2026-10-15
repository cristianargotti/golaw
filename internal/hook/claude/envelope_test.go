package claude_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/model"
)

//nolint:unparam // The shared frame builder keeps the event parameter for post-event callers.
func frame(t *testing.T, event, tool, cwd string, input any) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"hook_event_name": event, "tool_name": tool, "cwd": cwd,
		"session_id": "session", "agent_id": "agent", "tool_use_id": "tool",
		"tool_input": input,
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestHeaderFixtures(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/envelopes.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []json.RawMessage
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, data := range cases {
		got, err := claude.Header(data)
		if err != nil || got.Agent != "claude" || got.Cwd != "/project" ||
			got.SessionID != "session" || got.AgentID != "agent" || got.ToolUseID != "tool" {
			t.Fatalf("%s: %+v, %v", data, got, err)
		}
	}
}

func TestHeaderForeignAndMalformed(t *testing.T) {
	t.Parallel()
	for _, data := range []string{
		"", "{", "null", "[]", `{"hook_event_name":"PreToolUse"}`,
		`{"hook_event_name":"Other","tool_name":"Write","cwd":"/project","tool_input":{}}`,
		`{"hook_event_name":"PreToolUse","tool_name":"Write","cwd":"/project","tool_input":null}`,
	} {
		if _, err := claude.Header([]byte(data)); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatalf("%q: %v", data, err)
		}
	}
	for _, key := range []string{
		"conversation_id", "generation_id", "workspace_roots", "cursor_version",
	} {
		var raw map[string]any
		if err := json.Unmarshal(frame(t, "PreToolUse", "Write", "/project",
			map[string]any{"file_path": "/project/a.go", "content": ""}), &raw); err != nil {
			t.Fatal(err)
		}
		raw[key] = nil
		data, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := claude.Header(data); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatalf("%s: %v", key, err)
		}
	}
}
