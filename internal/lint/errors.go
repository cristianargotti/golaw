package lint

import "errors"

// The sentinel errors of the lint package, matched with errors.Is.
var (
	// ErrFormat marks tool output golaw cannot read: a format it cannot
	// read is Unmeasured, never clean.
	ErrFormat = errors.New("tool output in an unrecognized format")
	// ErrToolFailed marks an exit status the tool uses for its own failure,
	// not for findings.
	ErrToolFailed = errors.New("tool failed")
	// ErrUnparsed marks a file the tool could not parse: it stays
	// Unmeasured while the other files' issues are kept (M6.12 risk 14).
	ErrUnparsed = errors.New("tool could not parse a file")
	// ErrRows marks a rule map row golaw cannot trust.
	ErrRows = errors.New("invalid rule map")
)
