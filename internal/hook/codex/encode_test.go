package codex_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestEncode(t *testing.T) {
	t.Parallel()
	pre := model.Event{Shape: model.ShapeWrite}
	post := model.Event{Shape: model.ShapePost, Post: &model.PostResult{}}
	tests := []struct {
		name   string
		event  model.Event
		result hook.Result
		stdout string
		stderr string
		code   int
	}{
		{"allow", pre, hook.Result{Outcome: hook.Allow}, "", "", 0},
		{"post-allow", post, hook.Result{Outcome: hook.Allow}, "", "", 0},
		{
			"deny", pre,
			hook.Result{Outcome: hook.Deny, Reason: "repair the file", Human: "corrija o arquivo"},
			"{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"repair the file\"},\"systemMessage\":\"corrija o arquivo\"}\n",
			"repair the file\n", 0,
		},
		{
			"cannot-judge", pre,
			hook.Result{Outcome: hook.Deny, CannotJudge: true, Reason: "cannot read target", Human: "não foi possível ler"},
			"{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"cannot read target\"},\"systemMessage\":\"não foi possível ler\"}\n",
			"cannot read target\n", 0,
		},
		{
			"empty-reason", pre,
			hook.Result{Outcome: hook.Deny, CannotJudge: true},
			"{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"cannot judge\"},\"systemMessage\":\"\"}\n",
			"cannot judge\n", 0,
		},
		{
			"post-cannot-judge", post,
			hook.Result{Outcome: hook.Deny, CannotJudge: true, Reason: "cannot decode result"},
			"", "cannot decode result\n", 2,
		},
		{
			"post-empty-reason", post,
			hook.Result{Outcome: hook.Deny, CannotJudge: true},
			"", "cannot judge\n", 2,
		},
		{
			"feedback", post,
			hook.Result{Outcome: hook.Feedback, Reason: "repair the file", Human: "corrija o arquivo"},
			"{\"decision\":\"block\",\"reason\":\"repair the file\",\"systemMessage\":\"corrija o arquivo\"}\n",
			"", 0,
		},
		{
			"pre-notice", pre,
			hook.Result{Outcome: hook.Allow, Notice: "the pipeline judges it", Human: "a validação remota verifica"},
			"{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"additionalContext\":\"the pipeline judges it\"},\"systemMessage\":\"a validação remota verifica\"}\n",
			"", 0,
		},
		{
			"post-notice", post,
			hook.Result{Outcome: hook.Allow, Notice: "the pipeline judges it", Human: "a validação remota verifica"},
			"{\"hookSpecificOutput\":{\"hookEventName\":\"PostToolUse\",\"additionalContext\":\"the pipeline judges it\"},\"systemMessage\":\"a validação remota verifica\"}\n",
			"", 0,
		},
		{
			"post-feedback-cannot-judge", post,
			hook.Result{Outcome: hook.Feedback, CannotJudge: true, Reason: "cannot decode result"},
			"", "cannot decode result\n", 2,
		},
		{
			"pre-feedback", pre,
			hook.Result{Outcome: hook.Feedback},
			"{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"cannot judge: feedback before execution\"},\"systemMessage\":\"\"}\n",
			"cannot judge: feedback before execution\n", 0,
		},
		{
			"invalid-outcome", pre,
			hook.Result{Outcome: hook.Outcome("other")},
			"{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"cannot judge: invalid hook outcome\"},\"systemMessage\":\"\"}\n",
			"cannot judge: invalid hook outcome\n", 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reply := codex.Encode(tt.event, tt.result)
			if string(reply.Stdout) != tt.stdout || reply.Stderr != tt.stderr ||
				reply.ExitCode != tt.code {
				t.Fatalf("unexpected reply: %+v, stdout=%s", reply, reply.Stdout)
			}
		})
	}
}

func TestReasonBytesArePreserved(t *testing.T) {
	t.Parallel()
	reason := "  first line\nsecond line  "
	for _, tt := range []struct {
		event model.Event
		code  int
	}{
		{model.Event{Shape: model.ShapeWrite}, 0},
		{model.Event{Shape: model.ShapePost, Post: &model.PostResult{}}, 2},
	} {
		reply := codex.Encode(tt.event, hook.Result{
			Outcome: hook.Deny, CannotJudge: true, Reason: reason,
		})
		if reply.Stderr != reason+"\n" || reply.ExitCode != tt.code {
			t.Fatalf("reason changed: %q, exit %d", reply.Stderr, reply.ExitCode)
		}
	}
}
