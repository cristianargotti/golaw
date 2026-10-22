package release

import "errors"

// ErrInput marks a release input the caller could not read: a file of the
// release directory or the running executable.
var ErrInput = errors.New("release input")
