package claude_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/claude"
	"github.com/cristianargotti/golaw/internal/model"
)

type goldenCase struct {
	Name        string          `json:"name"`
	Post        bool            `json:"post"`
	Failed      bool            `json:"failed"`
	Outcome     hook.Outcome    `json:"outcome"`
	CannotJudge bool            `json:"cannot_judge"`
	Reason      string          `json:"reason"`
	Human       string          `json:"human"`
	Notice      string          `json:"notice"`
	Stdout      json.RawMessage `json:"stdout"`
	Stderr      string          `json:"stderr"`
	Exit        int             `json:"exit"`
}

func goldenCases(t *testing.T) []goldenCase {
	t.Helper()
	data, err := os.ReadFile("testdata/replies.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []goldenCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func TestEncodeGolden(t *testing.T) {
	t.Parallel()
	for _, tc := range goldenCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			event := goldenEvent(tc.Post, tc.Failed)
			result := hook.Result{
				Outcome: tc.Outcome, Reason: tc.Reason,
				Human: tc.Human, CannotJudge: tc.CannotJudge, Notice: tc.Notice,
			}
			reply := claude.Encode(event, result)
			expected := expectedStdout(t, tc.Stdout)
			if !bytes.Equal(reply.Stdout, expected) || reply.Stderr != tc.Stderr || reply.ExitCode != tc.Exit {
				t.Fatalf("got %+v, expected %s %q %d", reply, expected, tc.Stderr, tc.Exit)
			}
			if !mutable(tc.Outcome, tc.CannotJudge, tc.Post) {
				return
			}
			result.Outcome = oppositeOutcome(tc.Outcome, tc.Post)
			result.Reason, result.Human = "forced", "Operação bloqueada."
			if mutated := claude.Encode(event, result); mutated.Stderr == reply.Stderr &&
				mutated.ExitCode == reply.ExitCode && bytes.Equal(mutated.Stdout, reply.Stdout) {
				t.Fatal("decision mutation survived the golden")
			}
		})
	}
}

// goldenEvent carries only the shape and the failure flag, the two facts
// Encode reads to choose its response surface.
func goldenEvent(post, failed bool) model.Event {
	event := model.Event{}
	if post {
		event.Shape = model.ShapePost
		event.Post = &model.PostResult{Failed: failed}
	}
	return event
}

// expectedStdout normalizes the golden to the compact line Encode writes,
// so the comparison is byte for byte; a null golden is no output.
func expectedStdout(t *testing.T, raw json.RawMessage) []byte {
	t.Helper()
	if string(raw) == "null" {
		return nil
	}
	var expected bytes.Buffer
	if err := json.Compact(&expected, raw); err != nil {
		t.Fatal(err)
	}
	if err := expected.WriteByte('\n'); err != nil {
		t.Fatal(err)
	}
	return expected.Bytes()
}

// mutable excludes feedback before execution: Encode refuses it as cannot
// judge, so it has no opposite decision to flip.
func mutable(outcome hook.Outcome, cannotJudge, post bool) bool {
	if cannotJudge || outcome != hook.Allow && outcome != hook.Deny && outcome != hook.Feedback {
		return false
	}
	return outcome != hook.Feedback || post
}

// oppositeOutcome flips an allow to feedback after execution, because a
// post event cannot deny a tool that already ran.
func oppositeOutcome(outcome hook.Outcome, post bool) hook.Outcome {
	if outcome != hook.Allow {
		return hook.Allow
	}
	if post {
		return hook.Feedback
	}
	return hook.Deny
}
