package goldentest

import "errors"

// ErrProof marks a proof manifest that cannot be read or decoded.
var ErrProof = errors.New("invalid proof manifest")

// ErrSlice marks a golden slice the harness cannot run: a module, a base
// or a command missing, or a step it does not know.
var ErrSlice = errors.New("invalid golden slice")

// ErrFills marks a recorded fills file the harness cannot trust: unreadable,
// an unknown key, a repeated id or an empty body.
var ErrFills = errors.New("invalid recorded fills")
