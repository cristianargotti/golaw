//go:build !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !windows

package runner

import (
	"errors"
	"fmt"
	"os"
	"runtime"
)

type lockState struct {
	file *os.File
}

func lockOpenFlags() int { return os.O_RDWR | os.O_CREATE }

func (l *Lock) tryLock() error {
	return fmt.Errorf("lock %s on %s: %w", l.state.file.Name(), runtime.GOOS, errors.ErrUnsupported)
}

func (l *Lock) unlock() error { return nil }
