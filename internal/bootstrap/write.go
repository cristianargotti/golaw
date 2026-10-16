package bootstrap

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/model"
)

// Current reads the policy file at the root: nil when it does not exist,
// ErrSymlink when it is a link, because init would otherwise write through
// the link to a path outside the repository, and ErrTooLarge when it is
// longer than the agent hooks read one before its first commit, so init
// never enrolls a policy they cannot read.
func Current(root string) ([]byte, error) {
	target := filepath.Join(root, PolicyFile)
	if err := refuseSymlink(target); err != nil {
		return nil, err
	}
	file, err := os.Open(target) //nolint:gosec // the path is the policy file of the root init resolved
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("bootstrap: read %s: %w", PolicyFile, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, model.MaxPolicyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("bootstrap: read %s: %w", PolicyFile, err)
	}
	if len(data) > model.MaxPolicyBytes {
		return nil, fmt.Errorf("bootstrap: %s of %s is %w, so init neither replaces nor enrolls it", PolicyFile, root, ErrTooLarge)
	}
	return data, nil
}

// Write replaces the policy file atomically: the bytes land in a temporary
// file beside it and a rename publishes them, so a reader never sees a
// half written policy and an interrupted init leaves the old file intact.
func Write(root string, data []byte) error {
	target := filepath.Join(root, PolicyFile)
	if err := refuseSymlink(target); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(root, PolicyFile+".*.tmp")
	if err != nil {
		return fmt.Errorf("bootstrap: create temporary file: %w", err)
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		return discard(name, tmp.Close(), fmt.Errorf("bootstrap: write %s: %w", PolicyFile, err))
	}
	if err := tmp.Close(); err != nil {
		return discard(name, nil, fmt.Errorf("bootstrap: close %s: %w", PolicyFile, err))
	}
	if err := os.Rename(name, target); err != nil {
		return discard(name, nil, fmt.Errorf("bootstrap: publish %s: %w", PolicyFile, err))
	}
	return nil
}

// discard removes the temporary file after a failure and returns the
// failure; the removal and close errors are secondary and joined.
func discard(name string, closeErr, err error) error {
	return errors.Join(err, closeErr, os.Remove(name))
}

// refuseSymlink fails when the path exists and is a symbolic link.
func refuseSymlink(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("bootstrap: inspect %s: %w", PolicyFile, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s", ErrSymlink, PolicyFile)
	}
	return nil
}
