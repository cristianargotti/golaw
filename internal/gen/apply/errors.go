package apply

import "errors"

// ErrOutside is a write whose path leaves the repository root, by a dot dot,
// an absolute path or an unclean spelling: the engine never writes there, so
// such a write is a defect of the plan and stops the run before any byte.
var ErrOutside = errors.New("write outside the repository")
