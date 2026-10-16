// Package assets validates the bundle before installation can begin.
package assets

import (
	"fmt"

	"github.com/cristianargotti/golaw"
)

// File retains source identity with the bytes being installed.
type File struct {
	Path string
	Data []byte
}

// Read preserves bytes because installed skill copies must remain identical.
func Read(path string) ([]byte, error) {
	data, err := golaw.FS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read asset %s: %w", path, err)
	}
	return data, nil
}

// Files prevents installation from bypassing manifest validation.
func Files() ([]File, error) {
	return Load(golaw.FS)
}
