package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

const postDeadlineNotice = "golaw did not measure the changes of this command: the hook reached its deadline first; the pipeline judges them."

// deadlineCheck is a check whose id sorts after the text family and that
// waits for its context to be done, so the deadline of a test reaches the
// hook after the text checks found what they find; entered records that
// the run got that far.
type deadlineCheck struct{ entered *bool }

func (deadlineCheck) Spec() model.Spec {
	return model.Spec{ID: "wait.deadline", Rule: "rules/0@1", Scope: model.ScopeEvent, Owner: model.OwnerCore, Proof: "wait.deadline"}
}

func (c deadlineCheck) Run(ctx context.Context, _ model.Input) (model.Result, error) {
	*c.entered = true
	<-ctx.Done()
	return model.Result{Status: model.Unmeasured("deadline: " + ctx.Err().Error())}, nil
}

// TestHookKeepsFindingsBeforeThePostDeadline drives shell events whose
// deadline falls after the text checks through the real binding: a post
// event, or the failure event of a command, that wrote a dash is fed back
// with the deadline notice appended, one that wrote nothing a check finds
// answers the notice alone, and a pre event the deadline overtakes stays a
// denial.
func TestHookKeepsFindingsBeforeThePostDeadline(t *testing.T) { //nolint:paralleltest // the budget measures the hook without competing binding tests
	agent := agentUnderTest()
	for _, tc := range []struct {
		name, event, content string
		finding              bool
	}{
		{"a post finding", "PostToolUse", "one \u2014 two\n", true},
		{"a failed command finding", "PostToolUseFailure", "one \u2014 two\n", true},
		{"a post without a finding", "PostToolUse", "one-two\n", false},
		{"a pre event", "PreToolUse", "one-two\n", false},
	} {
		e := enrolledHook(t)
		pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
		if pre.ExitCode != 0 || len(pre.Stdout) != 0 || pre.Stderr != "" {
			t.Fatalf("%s baseline: %+v", tc.name, pre)
		}
		writeFiles(t, e.root, map[string]string{"notes.md": tc.content})
		var entered bool
		e.deps = withDeadlineCheck(t, e.deps, &entered)
		event := shellEventAt(t, tc.event, e.root, "true")
		if tc.event == "PostToolUseFailure" {
			event = strings.Replace(event, `"hook_event_name":`, `"error":"command failed","hook_event_name":`, 1)
		}
		reply := hook.Run(t.Context(), agent.ID, strings.NewReader(event), agent.Adapter, e.deps)
		if !entered {
			t.Fatalf("%s did not reach the final check: %+v", tc.name, reply)
		}
		if tc.event == "PreToolUse" {
			if reply.ExitCode != 2 || len(reply.Stdout) != 0 || !strings.Contains(reply.Stderr, "deadline") {
				t.Errorf("a late pre event: %+v", reply)
			}
			continue
		}
		checkLatePostReply(t, reply, tc.event, tc.finding)
	}
}

// withDeadlineCheck gives the dependencies the catalog with deadlineCheck
// after it and a budget the hook spends inside that check.
func withDeadlineCheck(t *testing.T, deps hook.Dependencies, entered *bool) hook.Dependencies {
	t.Helper()
	registry, err := checks.Build(append(checks.Catalog(), deadlineCheck{entered: entered}))
	if err != nil {
		t.Fatal(err)
	}
	deps.Registry, deps.Budget = registry, 3*time.Second
	return deps
}

// checkLatePostReply holds the answer to a late post event: the finding of
// notes.md with the deadline notice appended, or the notice alone.
func checkLatePostReply(t *testing.T, reply hook.Reply, event string, finding bool) {
	t.Helper()
	if reply.ExitCode != 0 || reply.Stderr != "" {
		t.Fatalf("%s: %+v", event, reply)
	}
	if !finding {
		agentText, humanText := noticeOf(t, reply)
		if agentText != postDeadlineNotice || strings.Contains(string(reply.Stdout), `"decision"`) ||
			humanText != "golaw não mediu as mudanças deste comando, que o pipeline julga." {
			t.Errorf("a late post without a finding: %+v", reply)
		}
		return
	}
	var answer struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
		Human    string `json:"systemMessage"`
		Specific struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(reply.Stdout, &answer); err != nil {
		t.Fatal(err)
	}
	if event == "PostToolUseFailure" {
		if answer.Decision != "" || answer.Specific.Event != event {
			t.Errorf("a failed command: %+v", reply)
		}
		answer.Reason = answer.Specific.Context
	} else if answer.Decision != "block" {
		t.Errorf("a late post finding: %+v", reply)
	}
	want := "text.dashes: em dash (U+2014) in text; use a comma, a colon or a hyphen (notes.md:1) " + postDeadlineNotice
	if answer.Reason != want || answer.Human != "Corrija a verificação text.dashes no caminho notes.md:1 antes de continuar." {
		t.Errorf("%s finding and notice: %+v", event, reply)
	}
}
