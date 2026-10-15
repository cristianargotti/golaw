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

// goldenCase is one reply of testdata/replies.json.
type goldenCase struct {
	Name        string          `json:"name"`
	Post        bool            `json:"post"`
	Failed      bool            `json:"failed"`
	Outcome     hook.Outcome    `json:"outcome"`
	CannotJudge bool            `json:"cannot_judge"`
	Reason      string          `json:"reason"`
	Human       string          `json:"human"`
	Stdout      json.RawMessage `json:"stdout"`
	Stderr      string          `json:"stderr"`
	Exit        int             `json:"exit"`
}

// goldenCases reads the golden replies.
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
				Human: tc.Human, CannotJudge: tc.CannotJudge,
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

// goldenEvent is the event of a golden case: empty before execution, a
// post event with its failure flag after it.
func goldenEvent(post, failed bool) model.Event {
	event := model.Event{}
	if post {
		event.Shape = model.ShapePost
		event.Post = &model.PostResult{Failed: failed}
	}
	return event
}

// expectedStdout compacts the golden stdout; null is no output.
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

// mutable reports whether the golden case has an opposite decision to
// encode: a judged outcome, and feedback only after execution.
func mutable(outcome hook.Outcome, cannotJudge, post bool) bool {
	if cannotJudge || outcome != hook.Allow && outcome != hook.Deny && outcome != hook.Feedback {
		return false
	}
	return outcome != hook.Feedback || post
}

// oppositeOutcome flips a decision: an allow becomes the denial of its
// phase and a denial becomes an allow.
func oppositeOutcome(outcome hook.Outcome, post bool) hook.Outcome {
	if outcome != hook.Allow {
		return hook.Allow
	}
	if post {
		return hook.Feedback
	}
	return hook.Deny
}
