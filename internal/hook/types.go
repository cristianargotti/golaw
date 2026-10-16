package hook

import "github.com/cristianargotti/golaw/internal/model"

// Request keeps candidate bytes outside the normalized event.
type Request struct {
	Event   model.Event
	Content map[string][]byte
	Denial  string
	Hints   []string
}

// Outcome separates prevention from feedback after execution.
type Outcome string

// The wire adapter must preserve this distinction.
const (
	Allow    Outcome = "allow"
	Deny     Outcome = "deny"
	Feedback Outcome = "feedback"
)

// Result keeps the agent reason separate from the human channel.
type Result struct {
	Outcome     Outcome
	Reason      string
	Human       string
	CannotJudge bool
}

// Reply keeps stdout and stderr separate through command integration.
type Reply struct {
	Stdout   []byte
	Stderr   string
	ExitCode int
}

// Adapter prevents vendor wire formats from entering the engine.
type Adapter struct {
	Header func([]byte) (model.Event, error)
	Decode func([]byte, string, model.Snapshot) (Request, error)
	Encode func(model.Event, Result) Reply
}
