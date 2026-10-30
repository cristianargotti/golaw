package claude_test

import (
	"encoding/json"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/model"
)

var feedbackLines = []string{
	"a.py:3 ruff/S105 (python:S2068) Possible hardcoded secret",
	"a.py:9 surface.python-function-length function build is 41 lines, the limit is 40: give it one job.",
}

type claudeAnswer struct {
	Specific *struct {
		Event    string `json:"hookEventName"`
		Decision string `json:"permissionDecision"`
		Context  string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
	Human    string `json:"systemMessage"`
}

func decodeClaude(t *testing.T, reply hook.Reply) claudeAnswer {
	t.Helper()
	var answer claudeAnswer
	if reply.ExitCode != 0 || json.Unmarshal(reply.Stdout, &answer) != nil {
		t.Fatalf("reply = %+v", reply)
	}
	return answer
}

func TestEncodeGivesTheLinesAsContextAfterAWrite(t *testing.T) {
	t.Parallel()
	post := model.Event{Shape: model.ShapePost, Post: &model.PostResult{}}
	allow := decodeClaude(t, claude.Encode(post, hook.Result{Outcome: hook.Allow, Lines: feedbackLines, Human: "golaw enviou ao agente 2 apontamentos."}))
	if allow.Specific == nil || allow.Specific.Event != "PostToolUse" || allow.Specific.Context != feedbackLines[0]+"\n"+feedbackLines[1] ||
		allow.Decision != "" || allow.Specific.Decision != "" || allow.Human != "golaw enviou ao agente 2 apontamentos." {
		t.Fatalf("allow = %+v", allow)
	}
	noticed := decodeClaude(t, claude.Encode(post, hook.Result{Outcome: hook.Allow, Notice: "golaw did not measure a.sh.", Lines: feedbackLines[:1]}))
	if noticed.Specific == nil || noticed.Specific.Context != "golaw did not measure a.sh.\n"+feedbackLines[0] {
		t.Fatalf("notice and lines = %+v", noticed)
	}
	fed := decodeClaude(t, claude.Encode(post, hook.Result{Outcome: hook.Feedback, Reason: "goast.x: y (a.go)", Lines: feedbackLines[:1]}))
	if fed.Decision != "block" || fed.Reason != "goast.x: y (a.go)\n"+feedbackLines[0] {
		t.Fatalf("feedback = %+v", fed)
	}
}
