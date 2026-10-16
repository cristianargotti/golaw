package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// Base is what a decoder names the files of an event against: the root of
// the repository that governs it and the home directory the host expands
// a leading tilde of a tool's path to, empty when the process has none.
type Base struct {
	Root string
	Home string
}

// Relative keeps candidates inside the discovered repository: a name is
// the file it leads to through every link, its last one included, named
// from the root, which is a real path, so a name given through a linked
// working directory, through the home or through a link outside the
// repository lands where the write lands.
func (b Base) Relative(cwd, name string) (string, error) {
	path, err := b.place(cwd, name)
	if err != nil {
		return "", err
	}
	return local(filepath.Rel(resolvePath(b.Root), resolvePath(path)))
}

// Requested is the name a read asks for, from the root: the path below the
// outermost directory above it that is the root, whatever links lead
// there, spelled as given, so a link between the root and the name, a
// directory, the entry itself or a link back to the root, changes no
// element of the name below the repository boundary the rule reads.
func (b Base) Requested(cwd, name string) (string, error) {
	path, err := b.place(cwd, name)
	if err != nil {
		return "", err
	}
	rootInfo, err := os.Stat(b.Root)
	if err != nil {
		return "", fmt.Errorf("%w: repository root: %w", model.ErrCannotJudge, err)
	}
	boundary := ""
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if info, statErr := os.Stat(dir); statErr == nil && os.SameFile(info, rootInfo) {
			boundary = dir
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	if boundary == "" {
		return "", fmt.Errorf("%w: candidate is outside the repository", model.ErrCannotJudge)
	}
	return local(filepath.Rel(boundary, path))
}

// Resolve is the file a name leads to through every link, as an absolute
// path, for a read that leads out of the repository; a name from a home
// the process does not know stays under the working directory.
func (b Base) Resolve(cwd, name string) string {
	path, err := absolute(b.Home, cwd, name)
	if err != nil {
		path = filepath.Join(cwd, name)
	}
	return resolvePath(path)
}

// place is the path the host opens for a name of an event.
func (b Base) place(cwd, name string) (string, error) {
	if name == "" || !filepath.IsAbs(b.Root) || !filepath.IsAbs(cwd) {
		return "", fmt.Errorf("%w: invalid candidate location", model.ErrCannotJudge)
	}
	return absolute(b.Home, cwd, name)
}

// local turns a relative path into the slash separated name of a file of
// the repository, refusing the root itself and a path that leaves it.
func local(relative string, err error) (string, error) {
	if err != nil {
		return "", fmt.Errorf("%w: relative path: %w", model.ErrCannotJudge, err)
	}
	if relative == "." || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("%w: candidate is outside the repository", model.ErrCannotJudge)
	}
	return strings.ReplaceAll(filepath.ToSlash(relative), `\`, "/"), nil
}
