package gensupport

import "errors"

// ErrStatus marks an answer whose status the caller did not expect.
var ErrStatus = errors.New("unexpected http status")
