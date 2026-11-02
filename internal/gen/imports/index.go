package imports

import (
	_ "embed"
	"go/parser"
	"go/token"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// stdList is recorded from go list std at kit build time, because the hook
// may not run go: a lookup must answer from bytes the binary carries.
//
//go:embed std.txt
var stdList string

// Index maps a package name to the import paths that declare it, and an
// import path to its package name.
type Index struct {
	names map[string][]string
	paths map[string]string
}

// Std answers a fresh index of the standard library, so a caller's With
// never reaches another caller's index.
func Std() Index {
	x := Index{names: map[string][]string{}, paths: map[string]string{}}
	for line := range strings.Lines(stdList) {
		importPath, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		if ok {
			x.add(importPath, name)
		}
	}
	return x
}

// With answers a copy of the index that also knows the importable packages
// of the module the snapshot holds; the receiver is left as it was.
func (x Index) With(module string, s model.Snapshot) Index {
	out := Index{names: maps.Clone(x.names), paths: maps.Clone(x.paths)}
	for name, list := range out.names {
		out.names[name] = slices.Clone(list)
	}
	seen := map[string]bool{}
	for _, f := range s.Files() {
		dir := path.Dir(f.Path)
		if seen[dir] || !importable(f) {
			continue
		}
		seen[dir] = true
		name, ok := packageName(s, f.Path)
		if !ok {
			continue
		}
		importPath := module
		if dir != "." {
			importPath = module + "/" + dir
		}
		out.add(importPath, name)
	}
	return out
}

// importable keeps the files whose package another package may import:
// no test, fixture, vendored or tool-ignored directory.
func importable(f model.File) bool {
	if !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
		return false
	}
	if f.Class != model.ClassTracked && f.Class != model.ClassUntracked && f.Class != model.ClassGenerated {
		return false
	}
	for segment := range strings.SplitSeq(path.Dir(f.Path), "/") {
		if segment == "testdata" || segment == "vendor" || strings.HasPrefix(segment, "_") || strings.HasPrefix(segment, ".") && segment != "." {
			return false
		}
	}
	return true
}

// packageName reads the package clause alone; a main package is never
// importable, so it answers no name.
func packageName(s model.Snapshot, filePath string) (string, bool) {
	content, err := s.Read(filePath)
	if err != nil {
		return "", false
	}
	file, err := parser.ParseFile(token.NewFileSet(), filePath, content, parser.PackageClauseOnly)
	if err != nil || file.Name.Name == "main" {
		return "", false
	}
	return file.Name.Name, true
}

// Lookup answers the sorted import paths whose package is named name.
func (x Index) Lookup(name string) []string { return slices.Clone(x.names[name]) }

// Name answers the package name of an import path the index knows.
func (x Index) Name(importPath string) (string, bool) {
	name, ok := x.paths[importPath]
	return name, ok
}

// add keeps names sorted so a lookup answers in a stable order.
func (x Index) add(importPath, name string) {
	if _, known := x.paths[importPath]; known {
		return
	}
	x.paths[importPath] = name
	list := x.names[name]
	at, _ := slices.BinarySearch(list, importPath)
	x.names[name] = slices.Insert(list, at, importPath)
}
