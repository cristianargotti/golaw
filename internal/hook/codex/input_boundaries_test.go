package codex_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

// A command the two canonical tools do not carry as a string, and a post
// event of a tool the adapter never decodes, are cannot judge in an
// enrolled repository, never a silent allow.
func TestCommandAndPostToolBoundaries(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		seed string
		edit func(map[string]any)
		want hook.Outcome
	}{
		{"command absent", "pre-bash-plain.json", func(m map[string]any) {
			m["tool_input"] = map[string]any{"cmd": "go test ./..."}
		}, hook.Deny},
		{"command not a string", "pre-apply-patch-add.json", func(m map[string]any) {
			m["tool_input"] = map[string]any{"command": 1}
		}, hook.Deny},
		{"post tool unknown", "post-bash.json", func(m map[string]any) {
			m["tool_name"] = "spawn_agent"
		}, hook.Feedback},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := rewrite(t, commandPayload(t, tt.seed, "true"), tt.edit)
			_, result := hooktest.Evaluate(t, "codex", adapter(), data, nil)
			if !result.CannotJudge || result.Outcome != tt.want {
				t.Fatalf("want %s cannot judge, got %+v", tt.want, result)
			}
		})
	}
}

// An input over the bound is cannot judge until its parse is built, and a
// cannot-judge result after the tool answers exit 2 with a stderr line,
// the one answer Codex reads on that exit, before and after the answers
// are frozen.
func TestOversizedInputAndPostCannotJudgeStayClosed(t *testing.T) {
	t.Parallel()
	if _, err := codex.Oversized(bytes.NewReader([]byte("{}"))); !errors.Is(err, model.ErrCannotJudge) {
		t.Fatalf("oversized = %v", err)
	}
	result := hook.Result{Outcome: hook.Feedback, CannotJudge: true, Reason: "cannot judge: the post event"}
	reply := codex.Encode(model.Event{Shape: model.ShapePost}, result)
	if reply.ExitCode != 2 || strings.TrimSpace(reply.Stderr) == "" || !strings.HasSuffix(reply.Stderr, "\n") {
		t.Fatalf("post cannot judge answered %+v", reply)
	}
}
