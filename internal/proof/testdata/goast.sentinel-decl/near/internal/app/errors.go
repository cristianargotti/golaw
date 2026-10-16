package app

import "errors"

// ErrNotFound marks a missing record.
var ErrNotFound = errors.New("not found")
