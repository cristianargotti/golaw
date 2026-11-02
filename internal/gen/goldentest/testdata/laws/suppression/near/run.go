package run

import (
	"fmt"
	"os"
)

// Clean removes the temporary file of one run; the linters run clean on it.
func Clean(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("clean %s: %w", path, err)
	}
	return nil
}
