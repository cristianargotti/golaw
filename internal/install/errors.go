package install

import (
	"errors"
	"fmt"
)

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
	ErrNotBuilt   = errors.New("the installer of this agent is not built yet")
)

func changedArtifact(path string, cause error) error {
	return fmt.Errorf("artifact %s: %w", path, cause)
}
