package codex_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/hooktest"
)

func TestSharedReasonsMatchClaude(t *testing.T) {
	t.Parallel()
	other := hook.Adapter{
		Parse: claude.Parse, Encode: claude.Encode, Oversized: claude.Oversized,
	}
	for _, tt := range []struct {
		name    string
		command string
	}{
		{"bypass", "git commit --no-verify -m x"},
		{"secret", "cat .env"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := commandPayload(t, "pre-bash-plain.json", tt.command)
			_, ours := hooktest.Evaluate(t, "codex", adapter(), data, nil)
			_, theirs := hooktest.Evaluate(t, "claude", other, data, nil)
			if ours.Outcome != hook.Deny || theirs.Outcome != hook.Deny ||
				ours.Reason != theirs.Reason {
				t.Fatalf("reason mismatch: %q != %q", ours.Reason, theirs.Reason)
			}
		})
	}
	content := "hello " + string(rune(0x2014)) + " world\n"
	oursData := commandPayload(t, "pre-apply-patch-add.json",
		patch("*** Add File: notes.md\n+"+content))
	theirsData := rewrite(t, corpus(t, "pre-bash-plain.json"), func(m map[string]any) {
		m["tool_name"] = "Write"
		m["tool_input"] = map[string]any{
			"file_path": "/project/notes.md", "content": content,
		}
	})
	_, ours := hooktest.Evaluate(t, "codex", adapter(), oursData, nil)
	_, theirs := hooktest.Evaluate(t, "claude", other, theirsData, nil)
	if ours.Outcome != hook.Deny || theirs.Outcome != hook.Deny ||
		ours.Reason != theirs.Reason {
		t.Fatalf("write reason mismatch: %q != %q", ours.Reason, theirs.Reason)
	}
}
