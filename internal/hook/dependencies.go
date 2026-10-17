package hook

import (
	"context"
	"time"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// Repository carries discovery facts instead of inferring enrollment from a
// file: its root, its identity, whether its filesystem folds case, its
// enrollment and the HEAD commit discovery found, empty on an unborn
// branch.
type Repository struct {
	Root            string
	Identity        string
	CaseInsensitive bool
	Enrollment      model.Enrollment
	Head            string
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
// identity reads every file of the snapshot. Moved answers the paths that
// differ between two commits with the blob the earlier one held at each,
// for a command that moved HEAD, an empty commit name being the empty tree
// of an unborn branch. Commits answers the commits that are no merge
// reachable from now and not from since, every one of now when since is
// empty, the walk bounded at those committed at or after the second from,
// and Committer the identity git commits as in the repository, both asked
// only when a move of HEAD brought a path. Hashes answers the blob git
// would store for the bytes each path holds now, through the conversions
// git applies on the way in, for a path whose bytes are not the blob it
// held.
type Dependencies struct {
	Home      string
	Begin     func()
	Enroll    func(ctx context.Context, dir, cwd string) (Repository, error)
	Policy    func(context.Context, Repository) (*model.EffectivePolicy, error)
	Snapshot  func(context.Context, Repository) (model.Snapshot, error)
	Changes   func(ctx context.Context, repository Repository, also []string) (Changes, error)
	Moved     func(ctx context.Context, repository Repository, from, to string) (map[string]string, error)
	Commits   func(ctx context.Context, repository Repository, since, now string, from int64) ([]Commit, error)
	Committer func(ctx context.Context, repository Repository) (string, error)
	Hashes    func(ctx context.Context, repository Repository, paths []string) (map[string]string, error)
	Overlay   func(context.Context, model.Snapshot, []model.Candidate, map[string][]byte) (model.Snapshot, error)
	Registry  *engine.Registry
	State     Store
	Budget    time.Duration
}

// Commit is a commit git log named between two HEADs: its committer, name
// and email as git writes an identity before the time, the second it was
// committed and the paths it changed against its parent.
type Commit struct {
	Committer string
	Time      int64
	Paths     []string
}

// Changes is what git status reported changed in a repository: a snapshot
// of those paths and of the paths asked for beside them, the paths it
// named, a deleted one included, the HEAD commit it compared with, empty
// on an unborn branch, the blob that commit holds at each tracked path it
// named, and the paths whose only change git found is an edit of their
// bytes under their own mode. Beside them: the paths whose index entry
// differs from HEAD while their bytes on disk are that entry, as a merge
// leaves what it staged, the paths git left unmerged, and the commits
// MERGE_HEAD names while a merge is in progress, empty when none is.
type Changes struct {
	Snapshot model.Snapshot
	Paths    []string
	Head     string
	Blobs    map[string]string
	Edited   map[string]bool
	Staged   map[string]bool
	Unmerged map[string]bool
	Merge    string
}

type execution struct {
	repository  Repository
	policy      *model.EffectivePolicy
	base        *deferredSnapshot
	request     Request
	stateReason string
}
