package doctor

import "errors"

// Probe errors preserve unavailable evidence as a distinct state.
var (
	ErrEnvironment = errors.New("agent environment unavailable")
	ErrBinary      = errors.New("binary absent from agent search path")
)

// ErrVersion prevents malformed output from satisfying a version floor.
var ErrVersion = errors.New("invalid version")
