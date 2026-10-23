package codex

import (
	"bytes"
	"encoding/json"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// PatchGrammar is the Codex release whose apply_patch grammar the decoder
// reads, the version doctor compares with the installed one.
const PatchGrammar = "0.163.0"

// Parse decodes an event once: the envelope of D16, the header enrollment
// reads, with every file an apply_patch, before or after it ran, or a
// here-document patch writes exposed as a target so the core enrolls the
// patch where it writes, and the decoder of its request.
func Parse(data []byte) (hook.Parsed, error) {
	event, in, err := parse(data)
	if err != nil {
		return hook.Parsed{Event: event}, err
	}
	var targets []string
	switch {
	case in.command == nil:
	case event.Tool == patchTool:
		event.Target, targets = patchTargets(event.Cwd, *in.command)
	case event.Shape == model.ShapeShell:
		event.Target, targets = heredocTargets(event.Cwd, *in.command)
	}
	return hook.Parsed{Event: event, Targets: targets, Decode: func(base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
		return decode(event, in, base, snapshot)
	}}, nil
}

// parse reads the envelope, the header and the tool input of an event once.
func parse(data []byte) (model.Event, toolInput, error) {
	w, err := envelope(data)
	if err != nil {
		return refusedEvent(data), toolInput{}, err
	}
	event, err := header(w)
	if err != nil {
		return model.Event{}, toolInput{}, err
	}
	return event, readInput(w), nil
}

// refusedEvent is the event of an envelope golaw refuses, shaped by its
// hook_event_name alone: a refused PostToolUse gets the post cannot-judge
// answer, exit 2 with the reason on stderr, the one answer Codex reads
// after its tool ran, never the PreToolUse deny JSON it rejects there.
func refusedEvent(data []byte) model.Event {
	var named struct {
		Name string `json:"hook_event_name"`
	}
	if json.NewDecoder(bytes.NewReader(data)).Decode(&named) != nil || named.Name != postEvent {
		return model.Event{}
	}
	return model.Event{Shape: model.ShapePost}
}
