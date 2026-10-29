package cursor

import (
	"encoding/json"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// The texts the encoder writes when a result carries none it can answer
// with: a denial with an empty field reads as no reason at all, and an
// outcome the core never produces must never leave as an allow. The line
// for the person is pt-BR, the configured language of the human channel.
const (
	emptyReason      = "cannot judge"
	feedbackTooEarly = "cannot judge: feedback before execution"
	unknownOutcome   = "cannot judge: invalid hook outcome"
	fallbackHuman    = "A operação foi bloqueada: o golaw não conseguiu avaliá-la."
)

// permissionDocument is the answer of preToolUse, beforeShellExecution
// and beforeReadFile: the permission, and on a denial the line for the
// person and, where the event's schema has it, the English reason for the
// agent.
type permissionDocument struct {
	Permission string `json:"permission"`
	User       string `json:"user_message,omitempty"`
	Agent      string `json:"agent_message,omitempty"`
}

// postDocument is the answer of postToolUse and postToolUseFailure: the
// context the agent reads, absent on a clean event, which answers {}.
type postDocument struct {
	Context string `json:"additional_context,omitempty"`
}

// Encode answers a result in the schema of its event (contract D5), always
// with exit 0 and one JSON document, an allow too, because under
// failClosed Cursor reads an empty answer as a failure and blocks: the
// permission JSON before a tool runs, the additional_context of a post
// event after it. An event whose envelope was refused, a panic and the
// internal deadline carry no shape and answer the permission denial.
func Encode(event model.Event, result hook.Result) hook.Reply {
	if event.Shape == model.ShapePost {
		return postReply(result)
	}
	return permissionReply(event, result)
}

// permissionReply is the answer of an event before its tool runs; a
// beforeReadFile denial carries no agent_message, because its documented
// output has none and Cursor blocks on a field outside a hook's schema.
func permissionReply(event model.Event, result hook.Result) hook.Reply {
	document := permission(result)
	if event.Shape == model.ShapeRead {
		document = readDocument(document)
	}
	return encoded(document)
}

// permission is the decision a result gives an event before its tool
// runs: CannotJudge is read before the outcome, and anything but an allow
// is a denial, so no result the core never produces leaves as an allow.
func permission(result hook.Result) permissionDocument {
	if result.CannotJudge {
		return denied(result.Reason, result.Human)
	}
	switch result.Outcome {
	case hook.Allow:
		return permissionDocument{Permission: "allow"}
	case hook.Deny:
		return denied(result.Reason, result.Human)
	case hook.Feedback:
		return denied(feedbackTooEarly, "")
	default:
		return denied(unknownOutcome, "")
	}
}

// denied is the denial with the engine's reason and the person's line
// unchanged, or the generic text where the result left one blank.
func denied(reason, human string) permissionDocument {
	return permissionDocument{Permission: "deny", User: orElse(human, fallbackHuman), Agent: orElse(reason, emptyReason)}
}

// readDocument is the permission of beforeReadFile: the reason stays out,
// so read parity is held on the core's result, never on the wire.
func readDocument(document permissionDocument) permissionDocument {
	document.Agent = ""
	return document
}

// postReply is the answer of an event after its tool ran: the reason of
// a finding or a cannot-judge result, which already holds any notice the
// core joined to it, with the feedback lines after it, else its notice, or
// the notice and the lines of an allow, or nothing; Cursor cannot block a tool that ran, so the answer carries no
// permission field.
func postReply(result hook.Result) hook.Reply {
	var context string
	switch {
	case result.CannotJudge, result.Outcome == hook.Feedback, result.Outcome == hook.Deny:
		context = orElse(result.Blocking(), orElse(result.Notice, emptyReason))
	case result.Outcome == hook.Allow:
		context = result.Context()
	default:
		context = unknownOutcome
	}
	return encoded(postDocument{Context: context})
}

// orElse keeps a field the agent or the person reads from leaving blank,
// because a blank reason gives neither of them anything to act on.
func orElse(text, fallback string) string {
	if strings.TrimSpace(text) == "" {
		return fallback
	}
	return text
}

// encoded is a document with its line feed. A document of strings always
// encodes; should one ever fail, exit 2 with a reason is the one other
// answer Cursor documents as a block.
func encoded(document any) hook.Reply {
	data, err := json.Marshal(document)
	if err != nil {
		return hook.Reply{ExitCode: 2, Stderr: "cannot judge: answer encoding failed\n"}
	}
	return hook.Reply{Stdout: append(data, '\n')}
}
