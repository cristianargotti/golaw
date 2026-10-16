package claude

import (
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/model"
)

// Oversized reads the header of an event whose input is over the bound of
// the hook from the whole input, in bounded memory: the event name, the
// tool, the working directory, the identities and the path a write names,
// wherever the host writes them, the content of the write read past and
// never held. An input that is not one JSON object, a header without the
// event, the tool or the working directory, and a field of another agent
// cannot be judged.
func Oversized(input io.Reader) (model.Event, error) {
	got, err := skim(input)
	if err != nil {
		return model.Event{}, fmt.Errorf("%w: an input over the bound: %w", model.ErrCannotJudge, err)
	}
	fields := got.fields
	w := wire{
		Name: fields["hook_event_name"], Tool: fields["tool_name"], Cwd: fields["cwd"],
		Session: fields["session_id"], Agent: fields["agent_id"], Use: fields["tool_use_id"],
	}
	if got.foreign || w.Name == "" || w.Tool == "" || w.Cwd == "" {
		return model.Event{}, fmt.Errorf("%w: the header of an input over the bound is incomplete", model.ErrCannotJudge)
	}
	event := model.Event{
		Agent: "claude", Tool: w.Tool, Cwd: w.Cwd, Target: got.path,
		SessionID: w.Session, AgentID: w.Agent, ToolUseID: w.Use,
	}
	switch w.Name {
	case preEvent:
		event.Shape = preShape(w.Tool)
	case postEvent, failureEvent:
		event.Shape = model.ShapePost
		event.Post = &model.PostResult{Failed: w.Name == failureEvent}
	default:
		return model.Event{}, fmt.Errorf("%w: unsupported Claude event", model.ErrCannotJudge)
	}
	return event, nil
}

// headerKey reports a field of the envelope Oversized keeps.
func headerKey(name string) bool {
	switch name {
	case "hook_event_name", "tool_name", "cwd", "session_id", "agent_id", "tool_use_id":
		return true
	default:
		return false
	}
}

// foreignKey reports a field of the envelope of another agent.
func foreignKey(name string) bool {
	switch name {
	case "conversation_id", "generation_id", "workspace_roots", "cursor_version":
		return true
	default:
		return false
	}
}
