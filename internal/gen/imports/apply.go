package imports

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Fix is the outcome of Apply: the new bytes, the import paths added and
// removed, and the lines the fix could not decide with certainty.
type Fix struct {
	Content                []byte
	Added, Removed, Report []string
}

// plan is what Apply decided before it touches any byte.
type plan struct {
	add     []string
	remove  []*ast.ImportSpec
	removed []string
	report  []string
}

// The verdicts of one import spec.
const (
	verdictKeep = iota
	verdictUnknown
	verdictRemove
	verdictAmbiguous
)

// Apply removes the imports no selector names and adds the ones a selector
// needs that the index answers with exactly one path. A file it leaves as
// it was answers its original bytes, so the caller writes nothing.
func Apply(path string, content []byte, x Index) (Fix, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return Fix{}, fmt.Errorf("%w: %w", ErrParse, err)
	}
	p := planImports(file, selectorNames(file), declaredNames(file), x)
	if len(p.add) == 0 && len(p.remove) == 0 {
		return Fix{Content: content, Report: p.report}, nil
	}
	formatted, err := format.Source(edit(fset, file, content, p))
	if err != nil {
		return Fix{}, fmt.Errorf("%w: %s after the import fix: %w", ErrParse, path, err)
	}
	return Fix{Content: formatted, Added: p.add, Removed: p.removed, Report: p.report}, nil
}

func planImports(file *ast.File, used, local map[string]bool, x Index) plan {
	var p plan
	imported := map[string]bool{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil || importPath == "C" {
			continue
		}
		names, fixed := specNames(spec, importPath, x)
		for _, name := range names {
			imported[name] = true
		}
		if fixed {
			continue
		}
		switch decide(names, used, local) {
		case verdictUnknown:
			p.report = append(p.report, importPath+": package name unknown, left as it is")
		case verdictRemove:
			p.remove = append(p.remove, spec)
			p.removed = append(p.removed, importPath)
		case verdictAmbiguous:
			p.report = append(p.report, importPath+": unused under "+strings.Join(names, " and ")+", left as it is")
		}
	}
	p.missing(used, imported, local, x)
	return p
}

// missing adds the one path a used name maps to, and reports a name that
// maps to several, because choosing one would be a guess.
func (p *plan) missing(used, imported, local map[string]bool, x Index) {
	for _, name := range slices.Sorted(maps.Keys(used)) {
		if imported[name] || local[name] {
			continue
		}
		switch paths := x.Lookup(name); len(paths) {
		case 0:
		case 1:
			p.add = append(p.add, paths[0])
		default:
			p.report = append(p.report, name+": "+strings.Join(paths, " or "))
		}
	}
	slices.Sort(p.add)
}

// decide keeps a spec any of whose names the file uses or declares: a
// local name may hide a use the fix cannot see without types.
func decide(names []string, used, local map[string]bool) int {
	if len(names) == 0 {
		return verdictUnknown
	}
	for _, name := range names {
		if used[name] || local[name] {
			return verdictKeep
		}
	}
	if len(names) == 1 {
		return verdictRemove
	}
	return verdictAmbiguous
}
