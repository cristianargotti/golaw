package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	directoryMode = 0o700
	artifactMode  = 0o600
)

// WriteAtomic avoids exposing partially written JSON or plugin files.
func WriteAtomic(path string, data []byte) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, directoryMode); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	file, err := os.CreateTemp(parent, ".golaw-*")
	if err != nil {
		return fmt.Errorf("create artifact temporary file: %w", err)
	}
	name := file.Name()
	err = writeTemporary(file, data)
	if err != nil {
		return errors.Join(err, removeTemporary(name))
	}
	err = os.Rename(name, path)
	if err != nil {
		return errors.Join(fmt.Errorf("replace artifact: %w", err), removeTemporary(name))
	}
	return nil
}

func removeTemporary(name string) error {
	err := os.Remove(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove artifact temporary file: %w", err)
	}
	return nil
}

func writeTemporary(file *os.File, data []byte) error {
	steps := []struct {
		name string
		run  func() error
	}{
		{"protect artifact", func() error { return file.Chmod(artifactMode) }},
		{"write artifact", func() error { _, err := file.Write(data); return err }}, //nolint:wrapcheck // the loop names the step
		{"sync artifact", file.Sync},
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			return fmt.Errorf("%s: %w", step.name, errors.Join(err, file.Close()))
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close artifact: %w", err)
	}
	return nil
}
