package hook

import (
	"context"
	"time"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// Repository carries discovery facts instead of inferring enrollment from a file.
type Repository struct {
	Root            string
	Identity        string
	CaseInsensitive bool
	Enrollment      model.Enrollment
}

// Dependencies keep policy authority and snapshot ownership in their packages.
// Enroll discovers the repository that governs a directory, the one that
// holds the event's target, and receives the session's working directory
// apart, as the evidence of where the agent runs the hook. Home is the
// home directory of the hook process, which it shares with the host that
// runs it, empty when it has none: the host expands a leading tilde of a
// tool's path to it before it opens the file.
type Dependencies struct {
	Home     string
	Enroll   func(ctx context.Context, dir, cwd string) (Repository, error)
	Policy   func(context.Context, Repository) (*model.EffectivePolicy, error)
	Snapshot func(context.Context, Repository) (model.Snapshot, error)
	Overlay  func(context.Context, model.Snapshot, []model.Candidate, map[string][]byte) (model.Snapshot, error)
	Registry *engine.Registry
	State    Store
	Budget   time.Duration
}

type execution struct {
	repository  Repository
	policy      *model.EffectivePolicy
	base        model.Snapshot
	request     Request
	stateReason string
}
