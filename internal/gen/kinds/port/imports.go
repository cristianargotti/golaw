package port

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// importsFor resolves the package names a rendered file uses into the
// imports of the contract file, plus the fixed paths the file always needs.
// A package whose name differs from its path's last element resolves only
// through an explicit import name, since the engine never guesses a package
// clause it has not read.
func importsFor(p genkind.Port, module string, names []string, fixed ...string) ([]genkind.Import, error) {
	for _, imp := range p.Imports {
		if imp.Name == "." {
			return nil, fmt.Errorf("%w: %s dot imports %s: write the port by hand under golaw:handwritten", genkind.ErrCannotJudge, p.Path, imp.Path)
		}
	}
	out := make([]genkind.Import, 0, len(names)+len(fixed))
	for _, name := range names {
		imp, err := resolve(p, module, name)
		if err != nil {
			return nil, err
		}
		out = append(out, imp)
	}
	for _, path := range fixed {
		out = append(out, genkind.Import{Path: path})
	}
	slices.SortStableFunc(out, func(a, b genkind.Import) int { return cmp.Compare(a.Path, b.Path) })
	return slices.CompactFunc(out, func(a, b genkind.Import) bool { return a.Path == b.Path }), nil
}

// resolve answers the import that names one package in the contract file.
func resolve(p genkind.Port, module, name string) (genkind.Import, error) {
	if name == p.Package {
		return genkind.Import{Path: module + "/" + p.Dir}, nil
	}
	for _, imp := range p.Imports {
		if imp.Name != "_" && importName(imp) == name {
			return imp, nil
		}
	}
	return genkind.Import{}, fmt.Errorf("%w: port %s.%s names package %s, which the contract file does not import", genkind.ErrCannotJudge, p.Package, p.Name, name)
}
