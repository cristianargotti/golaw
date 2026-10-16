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

// cut is the prefix of an event that ends inside its body, an input that
// does not end.
func cut(event, body string) string {
	return event[:strings.Index(event, body)+len(body)/2]
}

// TestOversizedReadsTheHeaderOfTheWholeInput reads the event name, the
// tool, the working directory, the identities and the path of a write from
// the whole input, the tool use id after the tool input and the path after
// a content of any size included, a post event and its failure alike.
func TestOversizedReadsTheHeaderOfTheWholeInput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "big.md")
	body := strings.Repeat("a", 64)
	write := fmt.Sprintf(`{"file_path":%q,"content":%q}`, path, body)
	got, err := claude.Oversized(strings.NewReader(ordered("PreToolUse", root, write)))
	if err != nil || got.Shape != model.ShapeWrite || got.Target != path || got.Agent != "claude" ||
		got.Cwd != root || got.SessionID != "session" || got.AgentID != "agent" || got.Tool != "Write" || got.ToolUseID != "tool" {
		t.Fatalf("a write = %+v, %v", got, err)
	}
	for name, failed := range map[string]bool{"PostToolUse": false, "PostToolUseFailure": true} {
		got, err = claude.Oversized(strings.NewReader(ordered(name, root, write)))
		if err != nil || got.Shape != model.ShapePost || got.Post == nil || got.Post.Failed != failed || got.Target != path {
			t.Fatalf("%s = %+v, %v", name, got, err)
		}
	}
	large := strings.Repeat("b", model.MaxReadBytes+1)
	late := fmt.Sprintf(`{"content":%q,"extra":[1,{"x":"}"},true,null,-2.5e3],"file_path":"%sé"}`, large, path)
	if got, err = claude.Oversized(strings.NewReader(ordered("PreToolUse", root, late))); err != nil || got.Target != path+"é" {
		t.Fatalf("a path after the content = %+v, %v", got, err)
	}
}

// TestOversizedRefusesWhatIsNotOneHeader refuses an input without the
// header, a foreign envelope, an event the adapter does not answer and an
// input that is not one JSON object.
func TestOversizedRefusesWhatIsNotOneHeader(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	body := strings.Repeat("a", 64)
	write := fmt.Sprintf(`{"file_path":%q,"content":%q}`, filepath.Join(root, "big.md"), body)
	for name, input := range map[string]string{
		"no header":                         `{"tool_input":{"content":"` + body + `"}}`,
		"a foreign key":                     `{"conversation_id":"c",` + ordered("PreToolUse", root, write)[1:],
		"an unknown event":                  ordered("SessionStart", root, write),
		"a header of another type":          strings.Replace(ordered("PreToolUse", root, write), `"cwd":`+fmt.Sprintf("%q", root), `"cwd":7`, 1),
		"no object":                         `["PreToolUse"]`,
		"an input that does not end":        cut(ordered("PreToolUse", root, write), body),
		"a value after the object":          ordered("PreToolUse", root, write) + ` {}`,
		"a member without a colon":          `{"cwd" "x"}`,
		"a trailing comma":                  `{"cwd":"x",}`,
		"a bare word":                       `{"cwd":@}`,
		"a header field past the kept size": strings.Replace(ordered("PreToolUse", root, write), fmt.Sprintf("%q", root), fmt.Sprintf("%q", strings.Repeat("r", 70<<10)), 1),
		"an escape the input never ends":    `{"cwd":"x\`,
		"a nested value that never ends":    `{"tool_response":{"a":[1,"b",`,
		"an empty input":                    ``,
	} {
		if _, err := claude.Oversized(strings.NewReader(input)); !errors.Is(err, model.ErrCannotJudge) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestOversizedLeavesATargetOnlyAWriteNames answers a tool input that is
// no object, or an object without a path, with no target, so the hook
// refuses a write it cannot place.
func TestOversizedLeavesATargetOnlyAWriteNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, input := range map[string]string{
		"a tool input that is a string": `"text"`,
		"a tool input that is an array": `[{"file_path":"x"}]`,
		"an empty tool input":           `{}`,
		"a path that is no string":      `{"file_path":7}`,
	} {
		got, err := claude.Oversized(strings.NewReader(ordered("PreToolUse", root, input)))
		if err != nil || got.Target != "" || got.Shape != model.ShapeWrite {
			t.Errorf("%s = %+v, %v", name, got, err)
		}
	}
}
