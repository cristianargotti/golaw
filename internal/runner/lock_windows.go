//go:build windows

package runner

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

type lockState struct {
	file       *os.File
	overlapped syscall.Overlapped
}

func lockOpenFlags() int { return os.O_RDWR | os.O_CREATE }

func (l *Lock) tryLock() error {
	const exclusive, immediate, violation = 0x2, 0x1, syscall.Errno(33)
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")
	if err := proc.Find(); err != nil {
		return fmt.Errorf("find LockFileEx for %s: %w", l.state.file.Name(), err)
	}
	result, _, err := proc.Call(l.state.file.Fd(), exclusive|immediate, 0, 1, 0,
		uintptr(unsafe.Pointer(&l.state.overlapped))) //nolint:gosec // The synchronous Windows call consumes the address while the retained value remains alive.
	runtime.KeepAlive(l)
	if result != 0 {
		return nil
	}
	if errors.Is(err, violation) {
		return ErrLocked
	}
	return fmt.Errorf("LockFileEx %s: %w", l.state.file.Name(), err)
}

func (l *Lock) unlock() error {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("UnlockFileEx")
	if err := proc.Find(); err != nil {
		return fmt.Errorf("find UnlockFileEx for %s: %w", l.state.file.Name(), err)
	}
	result, _, err := proc.Call(l.state.file.Fd(), 0, 1, 0,
		uintptr(unsafe.Pointer(&l.state.overlapped))) //nolint:gosec // The synchronous Windows call consumes the address while the retained value remains alive.
	runtime.KeepAlive(l)
	if result == 0 {
		return fmt.Errorf("UnlockFileEx %s: %w", l.state.file.Name(), err)
	}
	return nil
}
