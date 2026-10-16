package claude_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/model"
)

// ordered is an event in the field order the host writes, the tool input
// near the end, so a prefix that ends inside the input keeps the header.
func ordered(event, cwd, input string) string {
	return fmt.Sprintf(`{"session_id":"session","transcript_path":"","cwd":%q,"permission_mode":"default","hook_event_name":%q,"tool_name":"Write","agent_id":"agent","tool_input":%s,"tool_use_id":"tool"}`,
		cwd, event, input)
}

// cut is the prefix of an event that ends inside its body, as the bound of
// the hook leaves it.
func cut(event, body string) []byte {
	return []byte(event[:strings.Index(event, body)+len(body)/2])
}

// TestOversizedReadsTheHeaderThePrefixHolds reads the event name, the tool,
// the working directory, the identities and the path of a write from a
// prefix that ends inside the content, a post event and its failure
// alike, and refuses a prefix without the header, a foreign envelope and
// an event the adapter does not answer.
func TestOversizedReadsTheHeaderThePrefixHolds(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "big.md")
	body := strings.Repeat("a", 64)
	write := fmt.Sprintf(`{"file_path":%q,"content":%q}`, path, body)
	got, err := claude.Oversized(cut(ordered("PreToolUse", root, write), body))
	if err != nil || got.Shape != model.ShapeWrite || got.Target != path || got.Agent != "claude" ||
		got.Cwd != root || got.SessionID != "session" || got.AgentID != "agent" || got.Tool != "Write" {
		t.Fatalf("a write = %+v, %v", got, err)
	}
	for name, failed := range map[string]bool{"PostToolUse": false, "PostToolUseFailure": true} {
		got, err = claude.Oversized(cut(ordered(name, root, write), body))
		if err != nil || got.Shape != model.ShapePost || got.Post == nil || got.Post.Failed != failed || got.Target != path {
			t.Fatalf("%s = %+v, %v", name, got, err)
		}
	}
	late := fmt.Sprintf(`{"content":%q,"file_path":%q}`, body, path)
	if got, err = claude.Oversized(cut(ordered("PreToolUse", root, late), body)); err != nil || got.Target != "" {
		t.Fatalf("a path after the cut = %+v, %v", got, err)
	}
	for name, prefix := range map[string]string{
		"no header":        `{"tool_input":{"content":"` + body,
		"a foreign key":    `{"conversation_id":"c",` + ordered("PreToolUse", root, write)[1:],
		"an unknown event": ordered("SessionStart", root, write),
		"no object":        `["PreToolUse"`,
	} {
		if _, err := claude.Oversized([]byte(prefix)); !errors.Is(err, model.ErrCannotJudge) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
