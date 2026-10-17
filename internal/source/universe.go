package source

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// binaryProbe is how many leading bytes decide that a file is binary.
const binaryProbe = 8000

// generatedHeader is the Go convention for generated files.
var generatedHeader = regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`)

// entry is one path of the universe as git listed it.
type entry struct {
	path    string // slash separated, relative to the root
	tracked bool   // git tracks it; otherwise it would add it
	dir     bool   // git listed a directory: a nested repository
	head    string // the blob HEAD holds at the path, when git status named it
	edited  bool   // git status found its bytes edited in the worktree under its own mode
}

// listUniverse runs git ls-files -z -t -c -o --exclude-standard under the
// root and parses the tagged entries: ? for a path git would add, any
// other tag for a path it tracks; a nested repository arrives with a
// trailing slash. The entries come back sorted and unique.
func listUniverse(ctx context.Context, root string) ([]entry, error) {
	out, err := git(ctx, root, "ls-files", "-z", "-t", "-c", "-o", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var entries []entry
	for raw := range bytes.SplitSeq(out, []byte{0}) {
		tag, rest, found := strings.Cut(string(raw), " ")
		if !found || rest == "" {
			continue
		}
		e := entry{path: rest, tracked: tag != "?"}
		if dir, isDir := strings.CutSuffix(rest, "/"); isDir {
			e.path, e.dir = dir, true
		}
		entries = append(entries, e)
	}
	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.path, b.path) })
	return slices.CompactFunc(entries, func(a, b entry) bool { return a.path == b.path }), nil
}

// The states of the directories above a listed path.
const (
	dirsPlain  = iota // every directory above the path is an ordinary one
	dirsLinked        // one is a symbolic link or cannot be inspected
	dirsGone          // one no longer exists
)

// inspect stats one listed path and reads it within the bound. A symbolic
// link, a directory (a submodule or a nested repository) and anything that
// is not a regular file get their class without a read, and so does a path
// under a linked directory; the bytes come back with true only for a
// regular file read whole. A tracked path gone from the working tree, or
// under a directory that is gone, is a deletion the next commit carries,
// not a file that cannot be read: it answers ClassUnknown and leaves the
// universe.
func inspect(root string, e entry, dirs map[string]int) inspection {
	got := inspection{file: model.File{Path: e.path}}
	file := &got.file
	if e.dir {
		file.Class = model.ClassGitlink
		return got
	}
	switch parentDirs(root, e.path, dirs) {
	case dirsGone:
		return got
	case dirsLinked:
		file.Class = model.ClassUnreadable
		return got
	}
	info, err := os.Lstat(OSPath(root, e.path))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		file.Class = model.ClassUnknown
	case err != nil:
		file.Class = model.ClassUnreadable
	case info.Mode()&fs.ModeSymlink != 0:
		file.Class, file.Size = model.ClassSymlink, info.Size()
	case info.IsDir():
		file.Class = model.ClassGitlink
	case !info.Mode().IsRegular():
		file.Class = model.ClassUnreadable
	default:
		got.file, got.content, got.read = readRegular(root, e, info.Size())
		if info.Size() > model.MaxReadBytes {
			got.stamp = statIdentity(info)
		}
	}
	return got
}

// present reports whether a listed path is in the universe without
// reading it, by the rules inspect follows: a path that is gone, or under
// a directory that is gone, is not; anything else, an unreadable path
// included, is.
func present(root string, e entry, dirs map[string]int) bool {
	if e.dir {
		return true
	}
	if parentDirs(root, e.path, dirs) == dirsGone {
		return false
	}
	_, err := os.Lstat(OSPath(root, e.path))
	return !errors.Is(err, fs.ErrNotExist)
}

// parentDirs answers the state of the directories above a listed path: a
// symbolic link or a directory that cannot be inspected, because Lstat and
// a read follow every component but the last, so a tracked path whose
// directory became a link would otherwise read bytes from wherever the
// link points; and a directory that is gone. The state per directory is
// kept in dirs, top-down, so each one is inspected once.
func parentDirs(root, rel string, dirs map[string]int) int {
	for i, c := range rel {
		if c != '/' {
			continue
		}
		dir := rel[:i]
		state, known := dirs[dir]
		if !known {
			info, err := os.Lstat(OSPath(root, dir))
			switch {
			case errors.Is(err, fs.ErrNotExist):
				state = dirsGone
			case err != nil || info.Mode()&fs.ModeSymlink != 0:
				state = dirsLinked
			default:
				state = dirsPlain
			}
			dirs[dir] = state
		}
		if state != dirsPlain {
			return state
		}
	}
	return dirsPlain
}

// readRegular reads a regular file within the bound; a file beyond it,
// by its stat or by what the read found, keeps the class its path gives
// and none of its bytes.
func readRegular(root string, e entry, size int64) (model.File, []byte, bool) {
	file := model.File{Path: e.path, Size: size}
	var content []byte
	if size <= model.MaxReadBytes {
		read, ok := readBounded(OSPath(root, e.path))
		if !ok {
			file.Class = model.ClassUnreadable
			return file, nil, false
		}
		content, file.Size = read, int64(len(read))
	}
	if file.Size > model.MaxReadBytes {
		file.Class = Classify(e.path, nil, e.tracked)
		return file, nil, false
	}
	file.Class = Classify(e.path, content, e.tracked)
	return file, content, true
}

// readBounded stops one byte past model.MaxReadBytes: the stat said the
// file fits, and a file that grew since then shows up as one byte too many
// instead of a read without a bound.
func readBounded(name string) ([]byte, bool) {
	f, err := os.Open(name) //nolint:gosec // the name is a listed path under the real root
	if err != nil {
		return nil, false
	}
	defer f.Close()
	content, err := io.ReadAll(io.LimitReader(f, model.MaxReadBytes+1))
	return content, err == nil
}

// Classify assigns the class of section 5.1 to a regular file from its
// path, its content and whether git tracks it, with the precedence of
// model.FileClass: binary before generated, generated before fixture,
// fixture before test, tracked or untracked for everything else.
func Classify(rel string, content []byte, tracked bool) model.FileClass {
	probe := content
	if len(probe) > binaryProbe {
		probe = probe[:binaryProbe]
	}
	switch {
	case bytes.IndexByte(probe, 0) >= 0:
		return model.ClassBinary
	case generatedHeader.Match(content):
		return model.ClassGenerated
	case strings.HasPrefix(rel, "testdata/") || strings.Contains(rel, "/testdata/"):
		return model.ClassFixture
	case strings.HasSuffix(rel, "_test.go"):
		return model.ClassTest
	case tracked:
		return model.ClassTracked
	default:
		return model.ClassUntracked
	}
}
