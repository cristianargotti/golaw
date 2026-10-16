package hook

import (
	"io"

	"github.com/cristianargotti/golaw/internal/model"
)

// Request keeps candidate bytes outside the normalized event. Gaps names
// what a decoder could not reconstruct within the read bound: the hook
// measures nothing more of such an event and answers it with a notice.
type Request struct {
	Event   model.Event
	Content map[string][]byte
	Denial  string
	Hints   []string
	Gaps    []Gap
}

// Gap is what an event left unmeasured, for the agent: a path, or the
// changes of the command when the path is empty, and why.
type Gap struct {
	Path  string
	Cause string
}

// Outcome separates prevention from feedback after execution.
type Outcome string

// The wire adapter must preserve this distinction.
const (
	Allow    Outcome = "allow"
	Deny     Outcome = "deny"
	Feedback Outcome = "feedback"
)

// Result keeps the agent reason separate from the human channel. Notice
// is what an allowed event left unmeasured, for the agent, with Human
// naming it to the person.
type Result struct {
	Outcome     Outcome
	Reason      string
	Human       string
	CannotJudge bool
	Notice      string
}

// Reply keeps stdout and stderr separate through command integration.
type Reply struct {
	Stdout   []byte
	Stderr   string
	ExitCode int
}

// Adapter prevents vendor wire formats from entering the engine. Parse
// decodes an event once: the header enrollment reads before any target is
// reconstructed, and the decoder that names the files of the event against
// the root of its repository and the home the host expands a tool's path
// from. Oversized, when set, reads the header of an event from the whole of
// an input over the bound, in bounded memory.
type Adapter struct {
	Parse     func([]byte) (Parsed, error)
	Encode    func(model.Event, Result) Reply
	Oversized func(io.Reader) (model.Event, error)
}

// Parsed is an event an adapter decoded once: its header, and the decoder
// of its request, which reads none of the bytes of the event again.
type Parsed struct {
	Event  model.Event
	Decode func(Base, model.Snapshot) (Request, error)
}
