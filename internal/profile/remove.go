package profile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
)

// Remove deletes everything installed under a name (D3): the current
// bundle, every stored version found in the profiles directory whether or
// not a record lists it, and the record, so a damaged record never blocks
// the way out. A name with none of them is ErrBundle. The pin of golaw.yml
// is never touched: a repository that pins the name meets the install
// command of A19 on its next governed operation.
func Remove(dirs policy.Dirs, name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	paths, err := installedPaths(dirs, name)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return bundlef("profile %s is not installed", name)
	}
	errs := make([]error, 0, len(paths))
	for _, path := range paths {
		errs = append(errs, removeDir(path))
	}
	return errors.Join(errs...)
}

// installedPaths lists what exists of a name: the current bundle, the
// record and the stored copies, which are matched by name and an at sign
// so a name that prefixes another never takes the other's copies.
func installedPaths(dirs policy.Dirs, name string) ([]string, error) {
	root := profilesRoot(dirs.Config)
	paths := []string{Dir(dirs.Config, name), RecordPath(dirs.State, name)}
	entries, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, bundlef(readCause, root, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), name+"@") {
			paths = append(paths, filepath.Join(root, entry.Name()))
		}
	}
	return slices.DeleteFunc(paths, absent), nil
}

// absent reports a path with nothing at it.
func absent(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, fs.ErrNotExist)
}
