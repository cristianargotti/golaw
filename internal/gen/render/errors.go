package render

import "errors"

// ErrTemplate marks a rendering that cannot answer complete, formatted Go:
// the run stops before any write, so no file is left with a hole in it.
var ErrTemplate = errors.New("kind template does not render")
