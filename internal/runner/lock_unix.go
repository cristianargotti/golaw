//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package runner

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

type lockState struct {
	file *os.File
}

func lockOpenFlags() int {
	return os.O_RDWR | os.O_CREATE | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
}

func (l *Lock) tryLock() error {
	err := syscall.Flock(int(l.state.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return ErrLocked
	}
	if err != nil {
		return fmt.Errorf("flock %s: %w", l.state.file.Name(), err)
	}
	return nil
}

func (l *Lock) unlock() error {
	if err := syscall.Flock(int(l.state.file.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("unlock %s: %w", l.state.file.Name(), err)
	}
	return nil
}
