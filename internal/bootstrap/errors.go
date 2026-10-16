package bootstrap

import "errors"

// The sentinels of init; every one is a usage error for the command.
var (
	// ErrNotRepository marks a path that is not the root of a git checkout.
	ErrNotRepository = errors.New("not a git repository root")
	// ErrDecisions marks a repository with both decision directories.
	ErrDecisions = errors.New("both docs/adr and docs/decisions exist, keep one")
	// ErrSymlink marks a symbolic link where init expects a regular file.
	ErrSymlink = errors.New("symbolic link refused")
	// ErrTooLarge marks a policy file longer than the agent hooks read.
	ErrTooLarge = errors.New("over the 1 MiB the agent hooks read before its first commit")
)
