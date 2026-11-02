package thirdparty

import "errors"

// The errors of a pinned generator, matched with errors.Is, so golaw gen
// maps each to its exit code without reading the text wrapped around it.
var (
	// ErrMissing is a generator the tools module cannot provide: golaw gen
	// --check reports its outputs unmeasured (exit 2) instead of a drift it
	// could not prove, and the reason names the command that installs it.
	ErrMissing = errors.New("pinned generator missing")
	// ErrFailed is a generator that ran and refused its inputs: golaw gen
	// answers exit 2 with the generator's own output, which names the line
	// of the contract to fix.
	ErrFailed = errors.New("pinned generator failed")
)
