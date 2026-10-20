// Package coverage keeps measurement and floor evaluation independent of command execution.
package coverage

import "errors"

var (
	// ErrProfile distinguishes malformed measurements from missed coverage floors.
	ErrProfile = errors.New("invalid cover profile")
	// ErrModule prevents evaluation without the module identity used by profiles.
	ErrModule = errors.New("go.mod has no module line")
)
