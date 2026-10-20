package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func writeLockPID(path string) error {
	if err := removeLockPID(path); err != nil {
		return err
	}
	// Exclusive creation never follows stale metadata to an unrelated file.
	file, err := os.OpenFile(path+".pid", os.O_WRONLY|os.O_CREATE|os.O_EXCL, lockMode) //nolint:gosec // Only the metadata beside the acquired lock is created.
	if err != nil {
		return fmt.Errorf("create lock PID %s: %w", path, err)
	}
	_, writeErr := file.WriteString(strconv.Itoa(os.Getpid()) + "\n")
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return fmt.Errorf("write lock PID %s: %w", path, err)
	}
	return nil
}

func removeLockPID(path string) error {
	if err := os.Remove(path + ".pid"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove lock PID %s: %w", path, err)
	}
	return nil
}

func readLockPID(path string) int {
	name := path + ".pid"
	if err := regularLock(name, nil); err != nil {
		return 0
	}
	// Root opens share deletion on Windows so diagnosis cannot obstruct release.
	root, err := os.OpenRoot(filepath.Dir(name))
	if err != nil {
		return 0
	}
	file, err := root.OpenFile(filepath.Base(name), lockOpenFlags() & ^(os.O_CREATE|os.O_RDWR), 0)
	closeErr := root.Close()
	if err != nil {
		return 0
	}
	defer file.Close()
	if closeErr != nil {
		return 0
	}
	// A decimal PID fits in 20 digits; longer metadata cannot identify a holder.
	const maxPIDBytes = 22
	data, err := io.ReadAll(io.LimitReader(file, maxPIDBytes))
	if err != nil || len(data) == maxPIDBytes {
		return 0
	}
	value := strings.TrimSuffix(string(data), "\n")
	if strings.ContainsAny(value, "+- \t\r\n") {
		return 0
	}
	pid, err := strconv.Atoi(value)
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}
