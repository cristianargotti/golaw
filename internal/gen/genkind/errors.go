package genkind

import "errors"

// The errors of a kind, matched with errors.Is, so the command maps each one
// to its exit code without reading the text a kind wraps around it.
var (
	// ErrCannotJudge is a generic, embedded or aliased port (M7.3.1): the
	// shape needs type information go/parser does not give, so the kind
	// refuses instead of guessing, and the command answers exit 2.
	ErrCannotJudge = errors.New("contract cannot be judged without types")
	// ErrExists is golaw new over a name the manifest already holds: exit 1
	// naming the file, because a second instance would overwrite owned code.
	ErrExists = errors.New("kind instance exists")
	// ErrRequest is a missing or malformed flag: exit 2, a usage error.
	ErrRequest = errors.New("invalid kind request")
)
