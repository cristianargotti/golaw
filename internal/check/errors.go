package check

import "errors"

// ErrGit marks a git answer golaw check cannot read as the change.
var ErrGit = errors.New("git could not name the change")
