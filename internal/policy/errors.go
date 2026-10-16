package policy

import "errors"

// ErrNotAtHead marks an enrolled repository whose branch has commits and
// whose HEAD holds no golaw.yml: the file in the checkout governs nothing
// until a commit holds it (A2), so the remedy is a commit, never a new
// enrollment.
var ErrNotAtHead = errors.New("golaw.yml is not at HEAD")
