package adapterkit

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// importName is the name an import is used by: the explicit one, else the
// last path element, or the one before it when the last is a major version.
func importName(imp genkind.Import) string {
	if imp.Name != "" {
		return imp.Name
	}
	parts := strings.Split(imp.Path, "/")
	last := parts[len(parts)-1]
	if len(parts) > 1 && majorVersion(last) {
		return parts[len(parts)-2]
	}
	return last
}

func majorVersion(elem string) bool {
	digits, ok := strings.CutPrefix(elem, "v")
	return ok && digits != "" && strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// Uses answers, sorted by path, the imports an expression names, so a
// rendered file imports what its types select and nothing more.
func Uses(imports []genkind.Import, exprs []string) []genkind.Import {
	var out []genkind.Import
	for _, imp := range imports {
		name := importName(imp)
		if slices.ContainsFunc(exprs, func(e string) bool { return selects(e, name) }) {
			out = append(out, imp)
		}
	}
	return sortImports(out)
}

// selects reports name followed by a dot at the start of expr or after a
// character that cannot end a longer name or a selector.
func selects(expr, name string) bool {
	word := name + "."
	for from := 0; ; {
		i := strings.Index(expr[from:], word)
		if i < 0 {
			return false
		}
		at := from + i
		if before, _ := utf8.DecodeLastRuneInString(expr[:at]); at == 0 || !nameRune(before) {
			return true
		}
		from = at + 1
	}
}

func nameRune(r rune) bool {
	return r == '_' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func sortImports(imports []genkind.Import) []genkind.Import {
	slices.SortFunc(imports, func(a, b genkind.Import) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return slices.Compact(imports)
}

// importsOf matches the selected package names against the contract's
// imports; a name the file does not import cannot be placed without types.
func importsOf(t Target, pkgs []string) ([]genkind.Import, error) {
	var out []genkind.Import
	for _, name := range pkgs {
		i := slices.IndexFunc(t.Port.Imports, func(imp genkind.Import) bool { return importName(imp) == name })
		if i < 0 {
			return nil, fmt.Errorf("%w: package %s of %s is not imported by %s", genkind.ErrCannotJudge, name, t.Port.Name, t.Port.Path)
		}
		out = append(out, t.Port.Imports[i])
	}
	return sortImports(out), nil
}
