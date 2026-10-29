package toolchain

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Materialize writes one configuration set of the kit into the tool
// directory and answers where it landed. A file whose bytes are already
// there is left alone, so a hook running at that moment never sees it
// rewritten.
func Materialize(toolsDir, kitVersion, set string, files fs.FS) (string, error) {
	if toolsDir == "" {
		return "", ErrNoToolsDir
	}
	if set == "" || strings.ContainsAny(set, `/\`) || strings.Contains(set, "..") {
		return "", fmt.Errorf("%w: set %q", ErrConfig, set)
	}
	tree, err := fs.Sub(files, set)
	if err != nil {
		return "", fmt.Errorf("%w: set %s: %w", ErrConfig, set, err)
	}
	dir := ConfigDir(toolsDir, kitVersion, set)
	err = fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		return materializeFile(tree, name, filepath.Join(dir, filepath.FromSlash(name)))
	})
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: set %s: %w", ErrConfig, set, err)
	}
	if err != nil {
		return "", fmt.Errorf("materialize %s: %w", set, err)
	}
	return dir, nil
}

func materializeFile(tree fs.FS, name, target string) error {
	data, err := fs.ReadFile(tree, name)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	existing, err := os.ReadFile(target) //nolint:gosec // the target sits in the kit's own tool directory
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
		return fmt.Errorf("config directory: %w", err)
	}
	return writeAtomic(target, data, fileMode)
}
