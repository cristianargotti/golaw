package setup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/install"
)

func snapshotHooks(directory string) ([]install.Change, error) {
	names := []string{"pre-commit", "commit-msg", "pre-push"}
	changes := make([]install.Change, 0, len(names))
	for _, name := range names {
		path := filepath.Join(directory, name)
		data, exists, err := readHook(path)
		if err != nil {
			return nil, err
		}
		changes = append(changes, install.Change{Path: path, Before: data, Existed: exists})
	}
	return changes, nil
}

func readHook(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect setup hook %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("setup hook %q is not a regular file; move it aside, then run golaw setup again: %w", path, install.ErrPath)
	}
	data, err := os.ReadFile(path) //nolint:gosec // Only the three resolved hook paths and their backups are read.
	if err != nil {
		return nil, false, fmt.Errorf("read setup hook %q: %w", path, err)
	}
	return data, true, nil
}

func checkHookInputs(changes []install.Change) error {
	for _, change := range changes {
		data, exists, err := readHook(change.Path)
		if err != nil {
			return err
		}
		if exists != change.Existed || !bytes.Equal(data, change.Before) {
			return fmt.Errorf("setup hook %q; plan setup again: %w", change.Path, install.ErrChanged)
		}
	}
	return nil
}

func completedHooks(changes []install.Change) ([]install.Change, []install.Artifact, error) {
	diff := make([]install.Change, 0, len(changes))
	artifacts := make([]install.Artifact, 0, len(changes))
	for _, change := range changes {
		data, exists, err := readHook(change.Path)
		if err != nil {
			return nil, nil, err
		}
		if !exists {
			return nil, nil, fmt.Errorf("lefthook did not create %q: %w", change.Path, ErrGitHooks)
		}
		artifacts = append(artifacts, install.Artifact{Path: change.Path, SHA256: install.Digest(data)})
		if !change.Existed || !bytes.Equal(change.Before, data) {
			change.After = data
			diff = append(diff, change)
		}
	}
	return diff, artifacts, nil
}

func writeHookOutput(ctx context.Context, out io.Writer, changes []install.Change) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := install.WriteDiff(out, changes); err != nil {
		return fmt.Errorf("write setup diff: %w", err)
	}
	for _, change := range changes {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !change.Existed {
			continue
		}
		backup := change.Path + ".old"
		matches, err := backupMatches(backup, change.Before)
		if err != nil {
			return err
		}
		if matches {
			if _, err := fmt.Fprintf(out, "Hook anterior preservado em %s\n", backup); err != nil {
				return fmt.Errorf("write setup backup: %w", err)
			}
		}
	}
	return nil
}

func backupMatches(path string, before []byte) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) || err == nil && !info.Mode().IsRegular() {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect setup backup %q: %w", path, err)
	}
	data, _, err := readHook(path)
	return bytes.Equal(data, before), err
}
