package proof

import "errors"

// The errors of the proof, matched with errors.Is.
var (
	// ErrManifest marks a manifest that cannot be read or that breaks its format.
	ErrManifest = errors.New("invalid proof manifest")
	// ErrFixture marks a fixture tree that cannot be loaded or is empty.
	ErrFixture = errors.New("invalid proof fixture")
)
