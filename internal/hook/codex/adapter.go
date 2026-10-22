package codex

import (
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// PatchGrammar is the Codex release whose apply_patch grammar the decoder
// reads, the version doctor compares with the installed one.
const PatchGrammar = "0.163.0"

// notBuilt is the cause of every answer the adapter does not give yet: the
// hook fails closed meanwhile.
func notBuilt() error {
	return fmt.Errorf("%w: the codex adapter is not built yet", model.ErrCannotJudge)
}

// Parse decodes an event once: the envelope of D16, the header enrollment
// reads, with every file an apply_patch or a here-document patch writes
// exposed as a target so the core enrolls the patch where it writes, and
// the decoder of its request.
func Parse(data []byte) (hook.Parsed, error) {
	event, in, err := parse(data)
	if err != nil {
		return hook.Parsed{}, err
	}
	var targets []string
	switch {
	case in.command == nil:
	case event.Shape == model.ShapeWrite:
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
		return model.Event{}, toolInput{}, err
	}
	event, err := header(w)
	if err != nil {
		return model.Event{}, toolInput{}, err
	}
	return event, readInput(w), nil
}

// Oversized parses an event over the bound of the hook in bounded memory.
func Oversized(io.Reader) (hook.Parsed, error) {
	return hook.Parsed{}, notBuilt()
}

// Encode answers a result in the shape the host reads: until the adapter
// is built, every result leaves as the cannot-judge answer, exit 2 with a
// stderr line that is never empty, because exit 2 with an empty stderr
// fails open on Codex.
func Encode(_ model.Event, result hook.Result) hook.Reply {
	reason := result.Reason
	if reason == "" {
		reason = notBuilt().Error()
	}
	return hook.Reply{ExitCode: 2, Stderr: reason + "\n"}
}
