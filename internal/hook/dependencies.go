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
type Dependencies struct {
	Enroll   func(context.Context, string) (Repository, error)
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
