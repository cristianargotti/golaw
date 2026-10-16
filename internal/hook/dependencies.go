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
// tool's path to it before it opens the file. Begin, when set, is called
// once before each event is evaluated, so a binding that keeps what git
// answered forgets what an earlier event of the same dependencies learned.
// Changes, when set, answers what git status reports changed, the paths
// of also read beside them: the content identity of a shell command reads
// only those, and a path clean at both ends is unchanged. Without it the
// identity reads every file of the snapshot. Blobs answers the blob a
// commit holds at each path, for a path whose status alone may have moved
// while HEAD moved too. Hashes answers the blob git would store for the
// bytes each path holds now, through the conversions git applies on the
// way in, for a path whose bytes are not the blob it held.
type Dependencies struct {
	Home     string
	Begin    func()
	Enroll   func(ctx context.Context, dir, cwd string) (Repository, error)
	Policy   func(context.Context, Repository) (*model.EffectivePolicy, error)
	Snapshot func(context.Context, Repository) (model.Snapshot, error)
	Changes  func(ctx context.Context, repository Repository, also []string) (Changes, error)
	Blobs    func(ctx context.Context, repository Repository, commit string, paths []string) (map[string]string, error)
	Hashes   func(ctx context.Context, repository Repository, paths []string) (map[string]string, error)
	Overlay  func(context.Context, model.Snapshot, []model.Candidate, map[string][]byte) (model.Snapshot, error)
	Registry *engine.Registry
	State    Store
	Budget   time.Duration
}

// Changes is what git status reported changed in a repository: a snapshot
// of those paths and of the paths asked for beside them, the paths it
// named, a deleted one included, the HEAD commit it compared with, empty
// on an unborn branch, the blob that commit holds at each tracked path it
// named, and the paths whose only change git found is an edit of their
// bytes under their own mode.
type Changes struct {
	Snapshot model.Snapshot
	Paths    []string
	Head     string
	Blobs    map[string]string
	Edited   map[string]bool
}

type execution struct {
	repository  Repository
	policy      *model.EffectivePolicy
	base        *deferredSnapshot
	request     Request
	stateReason string
}
