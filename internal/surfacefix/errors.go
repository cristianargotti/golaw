package surfacefix

import "errors"

// The failure modes of the package, matched with errors.Is.
var (
	// ErrNotEmpty marks a Write into a directory that holds an entry other
	// than the .git directory of a new repository.
	ErrNotEmpty = errors.New("fixture directory is not empty")
	// ErrTwin marks a twin that is neither Red nor Corrected.
	ErrTwin = errors.New("unknown fixture twin")
)
