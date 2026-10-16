package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// TestHookValidatesAnEventOverTheBound drives events over the input bound
// through the real binding: JSON the decoders refuse within the bound, an
// unknown field of a write and a failure event of a write tool are cannot
// judge, NotebookEdit is denied for compatibility as at any size, and the
// same write made valid is allowed with the notice.
func TestHookValidatesAnEventOverTheBound(t *testing.T) { //nolint:paralleltest // an event of 8 MiB runs alone, so its deadline measures the hook, not the tests beside it
	e := enrolledHook(t)
	agent := agentUnderTest()
	big := strings.Repeat("a", hook.MaxInputBytes)
	path := fmt.Sprintf("%q", filepath.Join(e.root, "data.md"))
	event := func(name, tool, input string) string {
		return fmt.Sprintf(`{"session_id":"session","cwd":%q,"hook_event_name":%q,"tool_name":%q,"tool_input":%s,"tool_use_id":"bounds"}`, e.root, name, tool, input)
	}
	for _, tc := range []struct{ name, event, want string }{
		{"a raw newline", event("PreToolUse", "Write", `{"file_path":`+path+`,"content":"`+big+"\n"+`"}`), "cannot"},
		{"an unknown escape", event("PreToolUse", "Write", `{"file_path":`+path+`,"content":"`+big+`\q"}`), "cannot"},
		{"an unknown field", event("PreToolUse", "Write", `{"file_path":`+path+`,"content":"`+big+`","unknown":true}`), "cannot"},
		{"a failure of a write tool", strings.Replace(event("PostToolUseFailure", "Write", `{"file_path":`+path+`,"content":"`+big+`"}`), `,"tool_use_id"`, `,"error":"failed","tool_use_id"`, 1), "cannot"},
		{"NotebookEdit", event("PreToolUse", "NotebookEdit", `{"notebook_path":`+path+`,"new_source":"`+big+`"}`), "deny"},
		{"a valid write", event("PreToolUse", "Write", `{"file_path":`+path+`,"content":"`+big+`"}`), "notice"},
	} {
		reply := hook.Run(t.Context(), agent.ID, strings.NewReader(tc.event), agent.Adapter, e.deps)
		said := string(reply.Stdout)
		got := "allow"
		switch {
		case reply.ExitCode == 2:
			got = "cannot"
		case strings.Contains(said, `"permissionDecision":"deny"`):
			got = "deny"
		case strings.Contains(said, "golaw did not measure data.md"):
			got = "notice"
		}
		if got != tc.want {
			t.Errorf("%s: want %s, got %s: %+v", tc.name, tc.want, got, reply)
		}
	}
}
