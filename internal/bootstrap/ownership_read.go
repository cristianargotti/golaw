package bootstrap

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/cristianargotti/golaw/internal/model"
)

// ReadOwnership keeps checkout ownership reads within the same paths init can safely inspect.
func ReadOwnership(root string) ([]byte, error) {
	resolved, err := templateRoot(root)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"} {
		data, readErr := readOwnershipFile(resolved, name)
		if errors.Is(readErr, fs.ErrNotExist) {
			continue
		}
		return data, readErr
	}
	return nil, nil
}

func readOwnershipFile(root, name string) ([]byte, error) {
	path, err := templatePath(root, name)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path) //nolint:gosec // Every component was checked below the canonical root.
	if err != nil {
		return nil, fmt.Errorf("bootstrap: read ownership %q: %w", name, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, model.MaxPolicyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("bootstrap: read ownership %q: %w", name, err)
	}
	if len(data) > model.MaxPolicyBytes {
		return nil, fmt.Errorf("bootstrap: ownership %q: %w", name, ErrTooLarge)
	}
	return data, nil
}
