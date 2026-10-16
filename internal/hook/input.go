package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/cristianargotti/golaw/internal/model"
)

// MaxInputBytes bounds allocation before an envelope is trusted.
const MaxInputBytes = 1 << 20

// ReadObject requires EOF so another JSON value cannot hide after the event.
func ReadObject(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read input: %w", model.ErrCannotJudge, err)
	}
	if len(data) > MaxInputBytes {
		return nil, fmt.Errorf("%w: input exceeds 1 MiB", model.ErrCannotJudge)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON object: %w", model.ErrCannotJudge, err)
	}
	if object == nil {
		return nil, fmt.Errorf("%w: input must be one object", model.ErrCannotJudge)
	}
	return data, nil
}

// maxLinks bounds the links one name may pass through, as the system's
// own resolution does, so a loop of links ends.
const maxLinks = 255

// absolute is the path the host opens for a name: trimmed and cleaned the
// way the host expands a tool's path before it opens it (its parent
// elements resolved in the name, read in the 2.1.299 bundle), and joined to
// the working directory when it is relative.
func absolute(cwd, name string) string {
	path := filepath.FromSlash(strings.TrimFunc(name, hostSpace))
	if !filepath.IsAbs(path) {
		return filepath.Join(cwd, path)
	}
	return filepath.Clean(path)
}

// hostSpace is the white space the host trims from a path: the white space
// and line terminators of its JavaScript trim, which keeps U+0085 and
// trims U+FEFF.
func hostSpace(r rune) bool {
	return r == '\uFEFF' || unicode.IsSpace(r) && r != '\u0085'
}

// resolvePath follows every symbolic link of an absolute path, its last
// element included, the way the system resolves the path a write or read
// opens: a dangling link still names where a write would land, and the
// parent element of a link text is taken from where the link lands. Past
// an element that does not exist, or cannot be inspected, nothing can be
// followed, so the rest is kept as given; a loop of links stops the same way.
// A relative path has no place to start from and is kept as given.
func resolvePath(path string) string {
	if !filepath.IsAbs(path) {
		return path
	}
	vol := filepath.VolumeName(path)
	done, todo := vol+string(filepath.Separator), elements(path[len(vol):])
	for links := 0; len(todo) > 0; {
		name := todo[0]
		todo = todo[1:]
		if name == ".." {
			done = filepath.Dir(done)
			continue
		}
		next := filepath.Join(done, name)
		info, err := os.Lstat(next)
		if err == nil && info.Mode()&fs.ModeSymlink == 0 {
			done = next
			continue
		}
		text, linkErr := os.Readlink(next)
		if err != nil || linkErr != nil || links == maxLinks {
			return filepath.Join(append([]string{next}, todo...)...)
		}
		links++
		var rest string
		done, rest = rebase(done, text)
		todo = append(elements(rest), todo...)
	}
	return done
}

// rebase answers where the walk resumes after a link: at the root its text
// names when the text is rooted, else in the directory that holds the link.
func rebase(dir, text string) (string, string) {
	vol := filepath.VolumeName(text)
	rest := text[len(vol):]
	switch {
	case vol != "":
		return vol + string(filepath.Separator), rest
	case rest != "" && isSeparator(rune(rest[0])):
		return filepath.VolumeName(dir) + string(filepath.Separator), rest
	default:
		return dir, rest
	}
}

// elements splits a path into the names a walk visits, without the empty
// and the current directory ones.
func elements(path string) []string {
	return slices.DeleteFunc(strings.FieldsFunc(path, isSeparator), func(e string) bool { return e == "." })
}

func isSeparator(r rune) bool { return r == '/' || r == filepath.Separator }

// entryLocation is where the entry a name opens lives: the links of its
// directories followed, its last element kept, so a link is placed by the
// directory that holds it, not by where it leads.
func entryLocation(path string) string {
	return filepath.Join(resolvePath(filepath.Dir(path)), filepath.Base(path))
}

// existingDir is the deepest existing directory above a path, or the
// fallback when not even a root exists.
func existingDir(path, fallback string) string {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return fallback
		}
	}
}

// enrollmentDirs are where the enrollment of an event is looked up (A3),
// in order, the first enrolled one governing: for a write or a read, the
// directory above the file its name leads to through every link, then the
// one above the entry the name opens, so a link moves neither a write nor
// a read out of an enrolled repository in either direction; a read then
// falls back to the session's working directory. A shell command and an
// event without a target are judged where they run.
func enrollmentDirs(event model.Event) []string {
	if event.Target == "" {
		return []string{event.Cwd}
	}
	path := absolute(event.Cwd, event.Target)
	dirs := []string{existingDir(resolvePath(path), event.Cwd)}
	if entry := existingDir(entryLocation(path), event.Cwd); entry != dirs[0] {
		dirs = append(dirs, entry)
	}
	if event.Shape == model.ShapeRead && !slices.Contains(dirs, event.Cwd) {
		dirs = append(dirs, event.Cwd)
	}
	return dirs
}

// Hash binds a decision to bytes without retaining their contents.
func Hash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// Relative keeps candidates inside the discovered repository: a name is
// the file it leads to through every link, its last one included, named
// from the root, which is a real path, so a name given through a linked
// working directory or through a link outside the repository lands where
// the write lands.
func Relative(root, cwd, name string) (string, error) {
	if err := validLocation(root, cwd, name); err != nil {
		return "", err
	}
	return local(filepath.Rel(resolvePath(root), resolvePath(absolute(cwd, name))))
}

// Requested is the name a read asks for, from the root: the path below the
// deepest directory above it that is the root, whatever links lead there,
// spelled as given, so a link between the root and the name, a directory or
// the entry itself, changes no element the name rule reads.
func Requested(root, cwd, name string) (string, error) {
	if err := validLocation(root, cwd, name); err != nil {
		return "", err
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("%w: repository root: %w", model.ErrCannotJudge, err)
	}
	path := absolute(cwd, name)
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		if info, statErr := os.Stat(dir); statErr == nil && os.SameFile(info, rootInfo) {
			return local(filepath.Rel(dir, path))
		}
		if filepath.Dir(dir) == dir {
			return "", fmt.Errorf("%w: candidate is outside the repository", model.ErrCannotJudge)
		}
	}
}

// Resolve is the file a name leads to through every link, as an absolute
// path, for a read that leads out of the repository.
func Resolve(cwd, name string) string { return resolvePath(absolute(cwd, name)) }

func validLocation(root, cwd, name string) error {
	if name == "" || !filepath.IsAbs(root) || !filepath.IsAbs(cwd) {
		return fmt.Errorf("%w: invalid candidate location", model.ErrCannotJudge)
	}
	return nil
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
