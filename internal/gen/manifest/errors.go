package manifest

import (
	"errors"
	"fmt"
)

// ErrManifest marks a manifest golaw new and golaw gen would never write:
// the file is theirs alone (A42), so any such shape is a hand edit or a bad
// merge, and the remedy is to restore the file, never to guess its meaning.
var ErrManifest = errors.New("invalid generation manifest")

// invalid wraps ErrManifest so every refusal names the key or the entry the
// same way and callers match one sentinel.
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrManifest, fmt.Sprintf(format, args...))
}
