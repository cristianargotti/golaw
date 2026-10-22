//go:build unix

package doctor

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// detach starts codex app-server as the leader of a process group of its
// own, so ending the group ends whatever the server started.
func detach(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// endGroup kills the process group of the server, the server and every
// process it started; a group already gone is no error.
func endGroup(command *exec.Cmd) error {
	if command.Process == nil {
		return nil
	}
	if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("end the process group of codex app-server: %w", err)
	}
	return nil
}
