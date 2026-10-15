package install

import (
	"errors"
	"fmt"
)

//nolint:unused // These ownership keys are reserved for settings and receipt compatibility.
const (
	denyKey = "permissions/deny"
	kitName = "golaw"
)

// Installation errors distinguish invalid inputs from changed owned files.
var (
	ErrPath       = errors.New("invalid installation path")
	ErrSettings   = errors.New("invalid settings")
	ErrChanged    = errors.New("installation input changed")
	ErrEnrollment = errors.New("enrollment was not recorded")
	ErrHooks      = errors.New("invalid plugin hooks")
)

//nolint:unused // Artifact reads and writes wrap their failures with it.
func changedArtifact(path string, cause error) error {
	return fmt.Errorf("artifact %s: %w", path, cause)
}
