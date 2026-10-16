package main

import "errors"

// ErrStream marks a module stream the tool cannot read: the go command
// failed or printed something that is not a module object.
var ErrStream = errors.New("unreadable module stream")
