package setup

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/cristianargotti/golaw/internal/install"
)

// hookMode is the mode of a hook setup writes back after lefthook removed
// it; a hook it rewrites in place keeps the mode it has.
const hookMode fs.FileMode = 0o755

// restoreHooks puts each hook back as the plan found it after setup failed
// past lefthook install, and names the restoration in the error: a hook
// that did not exist is removed, a foreign hook lefthook moved aside comes
// back from its backup, and any other changed hook gets its bytes again.
func restoreHooks(changes []install.Change, cause error) error {
	var failed []error
	for _, change := range changes {
		if err := restoreHook(change); err != nil {
			failed = append(failed, err)
		}
	}
	if len(failed) != 0 {
		return fmt.Errorf("%w; setup could not put every git hook back, check them before running golaw setup again: %w", cause, errors.Join(failed...))
	}
	return fmt.Errorf("%w; setup put the git hooks back as they were before lefthook install", cause)
}

func restoreHook(change install.Change) error {
	data, exists, err := readHook(change.Path)
	if err != nil {
		return err
	}
	if exists == change.Existed && (!exists || bytes.Equal(data, change.Before)) {
		return nil
	}
	if !change.Existed {
		if removeErr := os.Remove(change.Path); removeErr != nil {
			return fmt.Errorf("remove the git hook %q: %w", change.Path, removeErr)
		}
		return nil
	}
	backup := change.Path + ".old"
	moved, err := backupMatches(backup, change.Before)
	if err != nil {
		return err
	}
	if moved {
		if renameErr := os.Rename(backup, change.Path); renameErr != nil {
			return fmt.Errorf("move the git hook %q back: %w", backup, renameErr)
		}
		return nil
	}
	if writeErr := os.WriteFile(change.Path, change.Before, hookMode); writeErr != nil {
		return fmt.Errorf("write the git hook %q back: %w", change.Path, writeErr)
	}
	return nil
}
