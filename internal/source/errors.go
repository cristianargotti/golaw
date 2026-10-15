package source

import "errors"

// The errors source raises beside the snapshot sentinels of model.
var (
	// ErrNotRepository marks a directory git does not recognize as a
	// repository: a scan outside git (A14).
	ErrNotRepository = errors.New("not a git repository")
	// ErrGit marks git plumbing that could not start or failed: the
	// discovery failure of A14 when it happens on an enrolled repository.
	ErrGit = errors.New("git plumbing failed")
	// ErrEscapedRoot marks a path that resolves outside the repository root.
	ErrEscapedRoot = errors.New("path escapes the repository root")
	// ErrUnsafeLink marks a path that traverses a symbolic link.
	ErrUnsafeLink = errors.New("path traverses a symbolic link")
	// ErrOverlay marks a candidate the overlay cannot apply: an unknown
	// operation, an edit of a path that is absent or not text, a move or a
	// deletion of an absent path.
	ErrOverlay = errors.New("invalid candidate")
	// ErrBaseMismatch marks a candidate bound to a content hash the
	// snapshot does not hold: a verdict computed on it would be stale.
	ErrBaseMismatch = errors.New("candidate base hash does not match the snapshot")
)
