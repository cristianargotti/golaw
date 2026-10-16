package model

import (
	"fmt"
	"regexp"
)

// agentPattern is the grammar of an agent id; the hook package owns the
// list of ids it answers and validates an event against it.
var agentPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// EventShape is the normalized form of a hook event (section 8.1); every
// rule runs on the shape, never on the vendor payload.
type EventShape int

// The four shapes of section 8.1.
const (
	ShapeWrite EventShape = iota + 1 // candidate files about to change
	ShapeRead                        // a file about to be read
	ShapeShell                       // a command about to run
	ShapePost                        // a tool that already ran
)

// Valid reports whether the shape is one of the four.
func (s EventShape) Valid() bool { return s >= ShapeWrite && s <= ShapePost }

// String names the shape; it is also the JSON form.
func (s EventShape) String() string {
	switch s {
	case ShapeWrite:
		return "write"
	case ShapeRead:
		return "read"
	case ShapeShell:
		return "shell"
	case ShapePost:
		return "post"
	default:
		return unknownName
	}
}

// MarshalText writes the shape name, so fixtures and reports read "write".
func (s EventShape) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// UnmarshalText reads a shape name; an unknown one is a usage error.
func (s *EventShape) UnmarshalText(text []byte) error {
	for candidate := ShapeWrite; candidate <= ShapePost; candidate++ {
		if candidate.String() == string(text) {
			*s = candidate
			return nil
		}
	}
	return fmt.Errorf("%w: unknown event shape %q", ErrUsage, string(text))
}

// CandidateOp is what a write does to one path.
type CandidateOp int

// The five candidate operations of section 8.1.
const (
	CandidateCreate  CandidateOp = iota + 1 // a path that does not exist yet
	CandidateReplace                        // the whole content of an existing path
	CandidateEdit                           // an edit applied to the content on disk
	CandidateDelete                         // the path goes away
	CandidateMove                           // the path moves from From
)

// Valid reports whether the operation is one of the five.
func (op CandidateOp) Valid() bool { return op >= CandidateCreate && op <= CandidateMove }

// String names the operation; it is also the JSON form.
func (op CandidateOp) String() string {
	switch op {
	case CandidateCreate:
		return "create"
	case CandidateReplace:
		return "replace"
	case CandidateEdit:
		return "edit"
	case CandidateDelete:
		return "delete"
	case CandidateMove:
		return "move"
	default:
		return unknownName
	}
}

// MarshalText writes the operation name.
func (op CandidateOp) MarshalText() ([]byte, error) { return []byte(op.String()), nil }

// UnmarshalText reads an operation name; an unknown one is a usage error.
func (op *CandidateOp) UnmarshalText(text []byte) error {
	for candidate := CandidateCreate; candidate <= CandidateMove; candidate++ {
		if candidate.String() == string(text) {
			*op = candidate
			return nil
		}
	}
	return fmt.Errorf("%w: unknown candidate operation %q", ErrUsage, string(text))
}

// Candidate is one file a write event is about to change. Its bytes travel
// in the snapshot overlay, never here, so a check reads them the same way it
// reads a committed file.
type Candidate struct {
	Path          string      `json:"path"`                // slash separated, relative to the repository root
	Op            CandidateOp `json:"op"`                  // what happens to the path
	From          string      `json:"from,omitempty"`      // the source path of a move
	Reconstructed bool        `json:"reconstructed"`       // the whole candidate content is known
	BaseHash      string      `json:"base_hash,omitempty"` // sha256 of the on-disk content an edit was applied to
}

// ReadRequest is a read event: the rule judges the name, never the bytes.
type ReadRequest struct {
	Path string `json:"path"` // as the agent gave it, normalized to slashes
}

// ShellCommand is a shell event.
type ShellCommand struct {
	Command string `json:"command"` // the command text, wrappers included
	Cwd     string `json:"cwd"`     // where it would run
	Tool    string `json:"tool"`    // the tool name the agent used: Bash, PowerShell, exec_command
}

// PostResult is a post event: detection after execution, never prevention.
type PostResult struct {
	Response string   `json:"response"`          // the tool response or failure text
	Failed   bool     `json:"failed"`            // the tool reported a failure
	Changed  []string `json:"changed,omitempty"` // paths whose content identity changed since the pre event
}

// Event is the normalized hook event. Exactly one payload matches Shape;
// the identity fields key the hook state of A15.
type Event struct {
	Shape      EventShape    `json:"shape"`
	Agent      string        `json:"agent"` // the agent id as the hook package names it
	Tool       string        `json:"tool"`
	Cwd        string        `json:"cwd,omitempty"`
	Target     string        `json:"target,omitempty"` // the path a write or read names, as given: enrollment is resolved from it (A3)
	SessionID  string        `json:"session_id,omitempty"`
	AgentID    string        `json:"agent_id,omitempty"`
	ToolUseID  string        `json:"tool_use_id,omitempty"`
	Candidates []Candidate   `json:"candidates,omitempty"`
	Read       *ReadRequest  `json:"read,omitempty"`
	Shell      *ShellCommand `json:"shell,omitempty"`
	Post       *PostResult   `json:"post,omitempty"`
}

// Validate refuses an event whose payload does not match its shape, so a
// decoder bug becomes a usage error instead of a silent allow.
func (e *Event) Validate() error {
	if e == nil {
		return fmt.Errorf("%w: nil event", ErrUsage)
	}
	if !e.Shape.Valid() {
		return fmt.Errorf("%w: event shape %d is not write, read, shell or post", ErrUsage, int(e.Shape))
	}
	if !agentPattern.MatchString(e.Agent) {
		return fmt.Errorf("%w: event agent %q is not a lower case id", ErrUsage, e.Agent)
	}
	present := map[EventShape]bool{
		ShapeWrite: len(e.Candidates) > 0,
		ShapeRead:  e.Read != nil,
		ShapeShell: e.Shell != nil,
		ShapePost:  e.Post != nil,
	}
	for shape, has := range present {
		if has != (shape == e.Shape) {
			return fmt.Errorf("%w: event shape %s does not match its payload", ErrUsage, e.Shape)
		}
	}
	for _, c := range e.Candidates {
		if c.Path == "" || !c.Op.Valid() || (c.Op == CandidateMove) != (c.From != "") {
			return fmt.Errorf("%w: candidate %q has an invalid operation", ErrUsage, c.Path)
		}
	}
	return nil
}
