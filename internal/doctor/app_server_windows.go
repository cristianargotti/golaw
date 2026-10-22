//go:build windows

package doctor

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// detach starts codex app-server in a process group of its own; ending
// the whole tree of a process on Windows needs a job object, for which
// D23 adds no module, so a child of the server is the limit
// docs/security.md names.
func detach(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// endGroup kills the server; a server already gone is no error.
func endGroup(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("end codex app-server: %w", err)
	}
	return nil
}
