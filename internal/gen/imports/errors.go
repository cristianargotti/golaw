package imports

import "errors"

// ErrParse marks a Go file the fix cannot read, or one the fix would leave
// unreadable; the hook names the parser line and writes nothing.
var ErrParse = errors.New("go file does not parse")
