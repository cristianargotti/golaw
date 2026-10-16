package hook

import (
	"bytes"
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

// MaxInputBytes bounds allocation before an envelope is trusted: the read
// bound of a file, so a write the snapshot could hold is judged whole.
const MaxInputBytes = model.MaxReadBytes

// inputBound is the cause of an input over the bound.
func inputBound() error {
	return fmt.Errorf("%w: %w: input exceeds 8 MiB", model.ErrCannotJudge, model.ErrTooLarge)
}

// ReadObject requires EOF so another JSON value cannot hide after the event.
// An input over the bound answers its first MaxInputBytes and one bytes
// and ErrTooLarge, the rest left in the reader for the caller, which reads
// the header from the whole input and drops the rest, so the host that
// writes it never meets a closed pipe. The input is validated in one scan
// that holds nothing, so the adapter's parse is its one decoding.
func ReadObject(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxInputBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read input: %w", model.ErrCannotJudge, err)
	}
	if len(data) > MaxInputBytes {
		return data, inputBound()
	}
	if !json.Valid(data) {
		// The decoder names what is wrong, which the scan does not.
		var value json.RawMessage
		return nil, fmt.Errorf("%w: invalid JSON object: %w", model.ErrCannotJudge, json.Unmarshal(data, &value))
	}
	if first := bytes.TrimLeft(data, " \t\r\n"); first[0] != '{' {
		return nil, fmt.Errorf("%w: input must be one object", model.ErrCannotJudge)
	}
	return data, nil
}

// maxLinks bounds the links one name may pass through, as the system's
// own resolution does, so a loop of links ends.
const maxLinks = 255

// absolute is the path the host opens for a name, expanded the way the
// host expands a tool's path before it opens it (read in the 2.1.299
// bundle): trimmed, a name that is ~ or starts with ~/ taken from the home,
// on Windows a name that starts with a slash, one letter and a slash read
// as that drive, its parent elements resolved, and joined to the working
// directory when it is relative. A name from a home the process does not
// know cannot be placed.
func absolute(home, cwd, name string) (string, error) {
	path := strings.TrimFunc(name, hostSpace)
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home == "" {
			return "", fmt.Errorf("%w: %s starts from a home this process does not know", model.ErrCannotJudge, path)
		}
		return filepath.Join(home, filepath.FromSlash(path[1:])), nil
	}
	if filepath.Separator == '\\' {
		path = windowsDrive(path)
	}
	path = filepath.FromSlash(path)
	if !filepath.IsAbs(path) {
		return filepath.Join(cwd, path), nil
	}
	return filepath.Clean(path), nil
}

// windowsDrive reads a name that starts with a slash, one letter and a
// slash as that drive, /c/work as C:\work, the way the host reads it on
// Windows.
func windowsDrive(name string) string {
	if len(name) < len("/c/") || name[0] != '/' || name[2] != '/' {
		return name
	}
	if letter := name[1]; (letter < 'a' || letter > 'z') && (letter < 'A' || letter > 'Z') {
		return name
	}
	return strings.ToUpper(name[1:2]) + ":" + strings.ReplaceAll(name[2:], "/", `\`)
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
	resolved, _ := resolveLinks(path)
	return resolved
}

// resolveLinks is resolvePath that also answers the directories holding
// the links the walk follows, in the order it meets them.
func resolveLinks(path string) (string, []string) {
	if !filepath.IsAbs(path) {
		return path, nil
	}
	var holders []string
	vol := filepath.VolumeName(path)
	done, todo := vol+string(filepath.Separator), elements(path[len(vol):])
	for len(todo) > 0 {
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
		if err != nil || linkErr != nil || len(holders) == maxLinks {
			return filepath.Join(append([]string{next}, todo...)...), holders
		}
		holders = append(holders, done)
		var rest string
		done, rest = rebase(done, text)
		todo = append(elements(rest), todo...)
	}
	return done, holders
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
// directory above the file its name, expanded as the host expands it,
// leads to through every link, then the one above the entry the name
// opens, then every directory that holds a link the name passes through,
// so a link, of a file or of a directory, moves neither a write nor a read
// out of an enrolled repository in either direction; a read then falls
// back to the session's working directory. A shell command and an event
// without a target are judged where they run.
func enrollmentDirs(event model.Event, home string) ([]string, error) {
	if event.Target == "" {
		return []string{event.Cwd}, nil
	}
	path, err := absolute(home, event.Cwd, event.Target)
	if err != nil {
		return nil, err
	}
	resolved, holders := resolveLinks(path)
	dirs := []string{existingDir(resolved, event.Cwd), existingDir(entryLocation(path), event.Cwd)}
	dirs = append(dirs, holders...)
	if event.Shape == model.ShapeRead {
		dirs = append(dirs, event.Cwd)
	}
	return distinct(dirs), nil
}

// distinct keeps the first of each directory, in order.
func distinct(dirs []string) []string {
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if !slices.Contains(out, dir) {
			out = append(out, dir)
		}
	}
	return out
}

// Hash binds a decision to bytes without retaining their contents.
func Hash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
