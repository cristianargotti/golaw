package codex

import (
	"encoding/json"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// The reasons the encoder writes when a result carries none it can block
// with, because Codex lets a tool run on a blocking answer whose reason is
// empty (survey section 3).
const (
	emptyReason      = "cannot judge"
	feedbackTooEarly = "cannot judge: feedback before execution"
	unknownOutcome   = "cannot judge: invalid hook outcome"
)

// specificOutput is the hookSpecificOutput of a Codex answer: a denial
// with its reason, or the context a notice adds; the generated output
// schemas of 0.163.0 refuse any other field.
type specificOutput struct {
	EventName string `json:"hookEventName"`
	Decision  string `json:"permissionDecision,omitempty"`
	Why       string `json:"permissionDecisionReason,omitempty"`
	Context   string `json:"additionalContext,omitempty"`
}

// answerDocument is the one JSON document of an answer: the specific
// output of a denial or a notice, or the block of a post finding, with the
// line for the person, which Codex shows as a warning.
type answerDocument struct {
	Specific *specificOutput `json:"hookSpecificOutput,omitempty"`
	Decision string          `json:"decision,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Message  string          `json:"systemMessage"`
}

// Encode answers a result the way Codex 0.163.0 reads it (contract section
// 2.3, as the owner's answer of 2026-10-21 to decision 2 amends it): an
// allow is empty; a pre denial and a pre cannot-judge exit 0 with the deny
// JSON and the reason on stderr, which blocks through every shell; a post
// finding is the block JSON; a post cannot-judge exits 2 with the reason on
// stderr, the one answer Codex reads after its tool ran; a notice is the
// context of its event. CannotJudge is read before the outcome, because a
// post rejection may carry Feedback.
func Encode(event model.Event, result hook.Result) hook.Reply {
	post := event.Shape == model.ShapePost
	if result.CannotJudge {
		return refusal(post, result.Reason, result.Human)
	}
	switch result.Outcome {
	case hook.Allow:
		if post && len(result.Lines) > 0 {
			// The post block replaces the tool's result with the feedback
			// (M6.6.3); the tool already ran, so it denies nothing.
			return encoded(answerDocument{Decision: "block", Reason: blocking(result.Context()), Message: result.Human})
		}
		if result.Notice == "" {
			return hook.Reply{}
		}
		return encoded(answerDocument{Specific: &specificOutput{EventName: eventName(post), Context: result.Notice}, Message: result.Human})
	case hook.Deny, hook.Feedback:
		if post {
			return encoded(answerDocument{Decision: "block", Reason: blocking(result.Blocking()), Message: result.Human})
		}
		if result.Outcome == hook.Feedback {
			return refusal(false, feedbackTooEarly, "")
		}
		return denial(result.Reason, result.Human)
	default:
		return refusal(post, unknownOutcome, "")
	}
}

// refusal is the cannot-judge answer of an event: the deny JSON before the
// tool runs, and after it the reason on stderr with exit 2.
func refusal(post bool, reason, human string) hook.Reply {
	if post {
		return hook.Reply{Stderr: blocking(reason) + "\n", ExitCode: 2}
	}
	return denial(reason, human)
}

// denial is the deny JSON of a pre event with the reason, byte for byte,
// on stderr as well, for the person who reads the terminal.
func denial(reason, human string) hook.Reply {
	why := blocking(reason)
	reply := encoded(answerDocument{Specific: &specificOutput{EventName: preEvent, Decision: "deny", Why: why}, Message: human})
	reply.Stderr = why + "\n"
	return reply
}

// blocking is the reason a blocking answer carries: the engine's own,
// unchanged, or the generic one when it is blank.
func blocking(reason string) string {
	if strings.TrimSpace(reason) == "" {
		return emptyReason
	}
	return reason
}

func eventName(post bool) string {
	if post {
		return postEvent
	}
	return preEvent
}

// encoded is a document with its line feed; a document that cannot be
// encoded leaves as exit 2 with a reason, which Codex reads as a block
// before its tool runs and as feedback after.
func encoded(document answerDocument) hook.Reply {
	data, err := json.Marshal(document)
	if err != nil {
		return hook.Reply{Stderr: "cannot judge: answer encoding failed\n", ExitCode: 2}
	}
	return hook.Reply{Stdout: append(data, '\n')}
}
