package genmap

import "errors"

// ErrNoArea is an area with no Go source under its port or use case
// directory.
var ErrNoArea = errors.New("no such area")
