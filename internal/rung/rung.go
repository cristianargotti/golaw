// Package rung runs the three optional git rungs of the ladder (section 7):
// pre-commit, commit-msg and pre-push. A rung runs only once a person armed
// it with golaw setup, switches off in the ladder of golaw.yml before any
// other work, refuses only what the commit or the push introduces, and
// answers git with an exit code and the lines a person reads; remote CI or
// the platform pipeline stays the only gate.
package rung

import (
	"context"
	"io"
	"time"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/policy"
)

// The names of the three git rungs, as golaw hook takes them and lefthook
// calls them.
const (
	PreCommit = "pre-commit"
	CommitMsg = "commit-msg"
	PrePush   = "pre-push"
)

// The codes a rung answers git, those of section 4.4.
const (
	codeClean    = 0
	codeFindings = 1
	codeFailure  = 2
)

// Names lists the rungs in the order git runs them: pre-commit,
// commit-msg, pre-push.
func Names() []string {
	return []string{PreCommit, CommitMsg, PrePush}
}

// Outcome is what a rung answers git.
type Outcome struct {
	Code  int      // 0 lets git go on, 1 refuses for enforced findings, 2 refuses for a failure or a required check left unmeasured
	Lines []string // printed on stdout: the English report lines, then, when Code is not 0, one line in the language of the policy
}

// Dependencies are what a rung needs from the process; cmd binds them.
type Dependencies struct {
	Dir      string    // the directory git runs the hook in: the worktree root
	Environ  []string  // the process environment, the hook variables of git included
	Now      time.Time // the clock of the process, read by cmd
	Load     func(ctx context.Context, dir string) (*policy.Loaded, error)
	Registry *engine.Registry
	Schemas  policy.Schemas
	State    string    // the state directory of policy.DefaultDirs: runner trees live under <State>/runner/trees
	Stdin    io.Reader // the ref lines of pre-push
}
