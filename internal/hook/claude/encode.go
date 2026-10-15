package claude

import (
	"encoding/json"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

type specificOutput struct {
	Event    string `json:"hookEventName"`
	Decision string `json:"permissionDecision,omitempty"`
	Reason   string `json:"permissionDecisionReason,omitempty"`
	Context  string `json:"additionalContext,omitempty"`
}

type output struct {
	Specific *specificOutput `json:"hookSpecificOutput,omitempty"`
	Decision string          `json:"decision,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Human    string          `json:"systemMessage,omitempty"`
}

// Encode preserves the reason while selecting the host response surface.
func Encode(event model.Event, result hook.Result) hook.Reply {
	if result.CannotJudge {
		return cannotReply(result.Reason)
	}
	switch result.Outcome {
	case hook.Allow:
		return hook.Reply{}
	case hook.Deny, hook.Feedback:
		if event.Shape == model.ShapePost {
			return postReply(event, result)
		}
		if result.Outcome == hook.Feedback {
			return cannotReply("cannot judge: feedback before execution")
		}
		return marshalReply(output{Specific: &specificOutput{
			Event: preEvent, Decision: "deny", Reason: result.Reason,
		}, Human: result.Human})
	default:
		return cannotReply("cannot judge: invalid hook outcome")
	}
}

func postReply(event model.Event, result hook.Result) hook.Reply {
	if event.Post != nil && event.Post.Failed {
		return marshalReply(output{Specific: &specificOutput{
			Event: failureEvent, Context: result.Reason,
		}, Human: result.Human})
	}
	return marshalReply(output{
		Decision: "block", Reason: result.Reason, Human: result.Human,
	})
}

func cannotReply(reason string) hook.Reply {
	if reason == "" {
		reason = "cannot judge"
	}
	return hook.Reply{ExitCode: 2, Stderr: reason + "\n"}
}

func marshalReply(document output) hook.Reply {
	data, err := json.Marshal(document)
	if err != nil {
		return cannotReply("cannot judge: response encoding failed")
	}
	return hook.Reply{Stdout: append(data, '\n')}
}
