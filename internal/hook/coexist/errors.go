package coexist

import "errors"

// ErrOwnedElsewhere marks a family the Python gate still owns on the
// repository, so arming the kit on it would be a second blocking verdict.
var ErrOwnedElsewhere = errors.New("the Python gate still owns a blocking verdict")

// ErrDetect marks a registration file the kit could not read or decode; a
// malformed registration is never silently "no gate".
var ErrDetect = errors.New("cannot read the gate registration")
