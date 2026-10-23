package cursor

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// The fields that carry what a tool left after it ran: the stringified
// result of a successful tool and the message of one that failed, timed
// out or was denied (survey section 8.4).
const (
	outputField = "tool_output"
	errorField  = "error_message"
)

// failureTypeField says why a postToolUseFailure tool did not finish;
// permissionDenied is a tool a hook or the person denied before it ran.
const (
	failureTypeField = "failure_type"
	permissionDenied = "permission_denied"
)

// postRequest is the request of a postToolUse or postToolUseFailure event
// of a Shell or Write tool, after the tool ran, with the identities the
// header read, the same the preToolUse event of the call kept its baseline
// under (A15), and the detection pass asked for, so the core judges what
// changed since that baseline whatever the tool reported. A failure is the
// event's name, never read from the text, and no file is named from the
// undocumented Write input: the detection pass alone finds the files.
func postRequest(input payload, event model.Event) (hook.Request, error) {
	if event.Tool != shellTool && event.Tool != writeTool {
		return hook.Request{}, fmt.Errorf("%w: unsupported cursor post tool %s", model.ErrCannotJudge, event.Tool)
	}
	failed := input.text(nameField) == failureEvent
	response, err := postResponse(input, failed)
	if err != nil {
		return hook.Request{}, err
	}
	event.Post = &model.PostResult{Response: response, Failed: failed}
	// A denied tool never ran, so there is nothing to detect, and the pre
	// event golaw denied kept no baseline to compare with (decision 13).
	if failed && input.text(failureTypeField) == permissionDenied {
		return hook.Request{Event: event}, nil
	}
	return hook.Request{Event: event, Detect: true}, nil
}

// postResponse is the documented string of a post event: the output of a
// success, the message of a failure. Either is required, as the Claude
// adapter requires its tool response, because a post event without it is
// not the event Cursor documents.
func postResponse(input payload, failed bool) (string, error) {
	if failed {
		return input.required(errorField)
	}
	return input.required(outputField)
}
