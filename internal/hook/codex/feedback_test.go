package codex_test

import (
	"encoding/json"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/model"
)

var feedbackLines = []string{
	"Dockerfile:12 hadolint/DL3008 Pin versions in apt get install.",
	"Dockerfile:1 docker.final-user (docker:S6471) the final stage sets no USER.",
}

type codexAnswer struct {
	Specific *struct {
		Event   string `json:"hookEventName"`
		Context string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
	Message  string `json:"systemMessage"`
}

func decodeCodex(t *testing.T, reply hook.Reply) codexAnswer {
	t.Helper()
	var answer codexAnswer
	if reply.ExitCode != 0 || json.Unmarshal(reply.Stdout, &answer) != nil {
		t.Fatalf("reply = %+v", reply)
	}
	return answer
}

func TestEncodeBlocksAPostEventWithTheLinesAsItsReason(t *testing.T) {
	t.Parallel()
	post := model.Event{Shape: model.ShapePost, Post: &model.PostResult{}}
	allow := decodeCodex(t, codex.Encode(post, hook.Result{Outcome: hook.Allow, Notice: "golaw did not measure a.sh.", Lines: feedbackLines, Human: "golaw enviou ao agente 2 apontamentos."}))
	if allow.Decision != "block" || allow.Reason != "golaw did not measure a.sh.\n"+feedbackLines[0]+"\n"+feedbackLines[1] || allow.Specific != nil || allow.Message != "golaw enviou ao agente 2 apontamentos." {
		t.Fatalf("allow with lines = %+v", allow)
	}
	notice := decodeCodex(t, codex.Encode(post, hook.Result{Outcome: hook.Allow, Notice: "golaw did not measure a.sh."}))
	if notice.Decision != "" || notice.Specific == nil || notice.Specific.Context != "golaw did not measure a.sh." {
		t.Fatalf("notice alone = %+v", notice)
	}
	fed := decodeCodex(t, codex.Encode(post, hook.Result{Outcome: hook.Feedback, Reason: "goast.x: y (a.go)", Lines: feedbackLines[:1]}))
	if fed.Decision != "block" || fed.Reason != "goast.x: y (a.go)\n"+feedbackLines[0] {
		t.Fatalf("feedback = %+v", fed)
	}
}
