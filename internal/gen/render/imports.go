package render

import (
	"cmp"
	"fmt"
	"go/ast"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// The gci sections of the lint asset, in the order the block writes them.
const (
	sectionStandard = iota
	sectionDefault
	sectionModule
)

// block builds the one import block of a rendered file from the template's
// specs and the extra ones, one spec per path, in the gci sections of the
// lint asset; an empty answer means the file imports nothing.
func block(file *ast.File, extra []genkind.Import, module string) (string, error) {
	all := make([]genkind.Import, 0, len(file.Imports)+len(extra))
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return "", fmt.Errorf("%w: import %s: %w", ErrTemplate, spec.Path.Value, err)
		}
		imp := genkind.Import{Path: importPath}
		if spec.Name != nil {
			imp.Name = spec.Name.Name
		}
		all = append(all, imp)
	}
	all = append(all, extra...)
	named := map[string]string{}
	var unique []genkind.Import
	for _, imp := range all {
		name, seen := named[imp.Path]
		if seen && name != imp.Name {
			return "", fmt.Errorf("%w: import %s named %q and %q", ErrTemplate, imp.Path, name, imp.Name)
		}
		if !seen {
			named[imp.Path] = imp.Name
			unique = append(unique, imp)
		}
	}
	return write(unique, module), nil
}

// write spells the sorted sections, one empty line between two of them.
func write(imports []genkind.Import, module string) string {
	if len(imports) == 0 {
		return ""
	}
	slices.SortFunc(imports, func(a, b genkind.Import) int {
		return cmp.Or(cmp.Compare(section(a.Path, module), section(b.Path, module)), cmp.Compare(a.Path, b.Path))
	})
	lines := []string{"import ("}
	for i, imp := range imports {
		if i > 0 && section(imports[i-1].Path, module) != section(imp.Path, module) {
			lines = append(lines, "")
		}
		line := "\t" + strconv.Quote(imp.Path)
		if imp.Name != "" {
			line = "\t" + imp.Name + " " + strconv.Quote(imp.Path)
		}
		lines = append(lines, line)
	}
	return strings.Join(append(lines, ")"), "\n") + "\n"
}

// section places a path: the standard library has no dot in its first
// element, the module is its own path or below it, the rest is default.
func section(importPath, module string) int {
	if importPath == module || strings.HasPrefix(importPath, module+"/") {
		return sectionModule
	}
	if first, _, _ := strings.Cut(importPath, "/"); !strings.Contains(first, ".") {
		return sectionStandard
	}
	return sectionDefault
}
