package codex_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
)

// The post event of a patch is enrolled where the patch wrote, as its pre
// event is, so a patch an agent applies from outside the repository has
// its files measured after it ran, never left unjudged by the working
// directory's unenrolled location.
func TestPostPatchIsJudgedWhereItWrote(t *testing.T) {
	t.Parallel()
	root, outside := hooktest.Repository(t), hooktest.Repository(t)
	inside := filepath.ToSlash(filepath.Join(root, "main.go"))
	for _, tt := range []struct {
		name  string
		lines int
		want  hook.Outcome
	}{
		{"red over the line limit", 301, hook.Feedback},
		{"near at the line limit", 300, hook.Allow},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := rewrite(t, corpus(t, "post-apply-patch.json"), func(value map[string]any) {
				value["cwd"] = filepath.ToSlash(outside)
				value["tool_input"] = map[string]any{"command": patch("*** Update File: " + inside + "\n@@\n package main\n+\n")}
			})
			parsed, err := codex.Parse(data)
			if err != nil || parsed.Event.Target != inside {
				t.Fatalf("target = %q, %v; want %q", parsed.Event.Target, err, inside)
			}
			files := map[string][]byte{"main.go": []byte("package main\n" + strings.Repeat("\n", tt.lines-1))}
			_, result := hook.Evaluate(t.Context(), "codex", data, adapter(), enrolledUnder(t, root, files))
			if result.Outcome != tt.want {
				t.Fatalf("want %s, got %+v", tt.want, result)
			}
		})
	}
}

// A post patch whose text does not parse is a decoder error after the
// tool ran: cannot judge, answered with a reason line, never an allow.
func TestPostPatchThatDoesNotParseCannotJudge(t *testing.T) {
	t.Parallel()
	data := commandPayload(t, "post-apply-patch.json", "*** Begin Patch\n*** Rename File: notes.md\n*** End Patch\n")
	_, result := hooktest.Evaluate(t, "codex", adapter(), data, nil)
	if !result.CannotJudge || result.Outcome == hook.Allow || !strings.Contains(result.Reason, "apply_patch line 2") {
		t.Fatalf("broken post patch: %+v", result)
	}
	if reply := hooktest.Answer(t, "codex", adapter(), data, nil); reply.ExitCode != 2 || strings.TrimSpace(reply.Stderr) == "" {
		t.Fatalf("broken post patch can fail open: %+v", reply)
	}
}
