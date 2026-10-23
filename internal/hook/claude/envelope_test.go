package claude_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/model"
)

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
		parsed, err := claude.Parse(data)
		got := parsed.Event
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
		if _, err := claude.Parse([]byte(data)); !errors.Is(err, model.ErrCannotJudge) {
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
		if _, err := claude.Parse(data); !errors.Is(err, model.ErrCannotJudge) {
			t.Fatalf("%s: %v", key, err)
		}
	}
}

// TestHeaderNamesTheTarget pins the path enrollment is resolved from: the
// file a write or read names, the notebook of NotebookEdit, and none for a
// shell tool or a path that is not a string.
func TestHeaderNamesTheTarget(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		event, tool string
		input       map[string]any
		want        string
	}{
		{"PreToolUse", "Write", map[string]any{"file_path": "/repo/a.go", "content": ""}, "/repo/a.go"},
		{"PreToolUse", "Edit", map[string]any{"file_path": "/repo/b.go"}, "/repo/b.go"},
		{"PreToolUse", "Read", map[string]any{"file_path": "/repo/notes.md"}, "/repo/notes.md"},
		{"PreToolUse", "NotebookEdit", map[string]any{"notebook_path": "/repo/c.ipynb", "file_path": "/other"}, "/repo/c.ipynb"},
		{"PostToolUse", "Edit", map[string]any{"file_path": "/repo/b.go"}, "/repo/b.go"},
		{"PreToolUse", "Bash", map[string]any{"command": "cat /repo/a.go", "file_path": "/repo/a.go"}, ""},
		{"PreToolUse", "Write", map[string]any{"file_path": 1}, ""},
	} {
		parsed, err := claude.Parse(frame(t, tc.event, tc.tool, "/project", tc.input))
		got := parsed.Event
		if err != nil || got.Target != tc.want || got.Cwd != "/project" {
			t.Fatalf("%s %s: %+v, %v", tc.event, tc.tool, got, err)
		}
	}
}

// TestHeaderShapesThePreEvent pins the shape a pre event takes from its
// tool, which enrollment reads before the decoder runs.
func TestHeaderShapesThePreEvent(t *testing.T) {
	t.Parallel()
	for tool, want := range map[string]model.EventShape{
		"Write": model.ShapeWrite, "Edit": model.ShapeWrite, "NotebookEdit": model.ShapeWrite,
		"Read": model.ShapeRead, "Bash": model.ShapeShell, "PowerShell": model.ShapeShell, "Glob": 0,
	} {
		parsed, err := claude.Parse(frame(t, "PreToolUse", tool, "/project", map[string]any{}))
		got := parsed.Event
		if err != nil || got.Shape != want {
			t.Fatalf("%s: %v, %v", tool, got.Shape, err)
		}
	}
	parsed, err := claude.Parse(frame(t, "PostToolUse", "Read", "/project", map[string]any{}))
	got := parsed.Event
	if err != nil || got.Shape != model.ShapePost {
		t.Fatalf("post: %v, %v", got.Shape, err)
	}
}
