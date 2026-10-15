package install

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

func readOptional(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect artifact: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, changedArtifact(path, ErrPath)
	}
	data, err := os.ReadFile(path) //nolint:gosec // Installation constructs the destination.
	if err != nil {
		return nil, false, fmt.Errorf("read artifact: %w", err)
	}
	return data, true, nil
}

func addChange(plan *Plan, path string, data []byte, remove bool) error {
	before, exists, err := readOptional(path)
	if err != nil {
		return err
	}
	if (remove && !exists) || (!remove && exists && bytes.Equal(before, data)) {
		return nil
	}
	plan.Changes = append(plan.Changes, Change{
		Path: path, Before: before, After: data, Existed: exists, Remove: remove,
	})
	return nil
}

// WriteDiff emits complete replacements before any mutation is possible.
func WriteDiff(out io.Writer, changes []Change) error {
	for _, change := range changes {
		if _, err := fmt.Fprintf(out, "--- %s\n+++ %s\n", change.Path, change.Path); err != nil {
			return fmt.Errorf("write diff header: %w", err)
		}
		if err := diffLines(out, "-", change.Before); err != nil {
			return err
		}
		if err := diffLines(out, "+", change.After); err != nil {
			return err
		}
	}
	return nil
}

func diffLines(out io.Writer, prefix string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	for line := range strings.SplitSeq(string(bytes.TrimSuffix(data, []byte("\n"))), "\n") {
		if _, err := fmt.Fprintln(out, prefix+line); err != nil {
			return fmt.Errorf("write diff line: %w", err)
		}
	}
	return nil
}
