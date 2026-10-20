package runner

import "errors"

// The sentinel errors of the runner, matched with errors.Is; a rung turns
// each into a reason that names the fix.
var (
	// ErrStart marks a program that could not start: absent from PATH or
	// not executable.
	ErrStart = errors.New("program could not start")
	// ErrToolMissing marks a pinned tool PATH does not hold.
	ErrToolMissing = errors.New("pinned tool not found")
	// ErrToolVersion marks a pinned tool whose probe names another version.
	ErrToolVersion = errors.New("pinned tool at another version")
	// ErrLocked marks a lock another process still holds after the wait.
	ErrLocked = errors.New("lock held by another process")
	// ErrBound marks a tree a materialization would write over its bound.
	ErrBound = errors.New("tree over the materialization bound")
	// ErrRefUpdate marks a line of pre-push input git never writes.
	ErrRefUpdate = errors.New("malformed pre-push ref line")
	// ErrPackages marks a go list that could not name the packages.
	ErrPackages = errors.New("go list could not name the packages")
)
