package source

import (
	"bytes"
	"context"
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

// inspect stats one listed path and reads it within the bound. A symbolic
// link, a directory (a submodule or a nested repository) and anything that
// is not a regular file get their class without a read; the bytes come
// back with true only for a regular file read whole.
func inspect(root string, e entry) (model.File, []byte, bool) {
	file := model.File{Path: e.path}
	if e.dir {
		file.Class = model.ClassGitlink
		return file, nil, false
	}
	info, err := os.Lstat(OSPath(root, e.path))
	switch {
	case err != nil:
		file.Class = model.ClassUnreadable
	case info.Mode()&fs.ModeSymlink != 0:
		file.Class, file.Size = model.ClassSymlink, info.Size()
	case info.IsDir():
		file.Class = model.ClassGitlink
	case !info.Mode().IsRegular():
		file.Class = model.ClassUnreadable
	default:
		return readRegular(root, e, info.Size())
	}
	return file, nil, false
}

// readRegular reads a regular file within the bound; a file beyond it
// keeps the class its path gives and is never read.
func readRegular(root string, e entry, size int64) (model.File, []byte, bool) {
	file := model.File{Path: e.path, Size: size}
	if size > model.MaxReadBytes {
		file.Class = Classify(e.path, nil, e.tracked)
		return file, nil, false
	}
	content, err := os.ReadFile(OSPath(root, e.path))
	if err != nil {
		file.Class = model.ClassUnreadable
		return file, nil, false
	}
	file.Size = int64(len(content))
	file.Class = Classify(e.path, content, e.tracked)
	return file, content, true
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
