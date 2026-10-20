//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package runner

import (
	"errors"
	"math"
	"syscall"
)

func treeProcessGone(pid int) bool {
	return pid > 0 && pid <= math.MaxInt32 && errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
}
