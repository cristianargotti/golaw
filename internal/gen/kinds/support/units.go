package support

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// rendered is one support file with the part of the package it belongs to.
type rendered struct {
	file genkind.File
	unit string
}

// unitOf answers the part of the support tree a template belongs to: a
// subpackage is one part, and a root file is the part its name starts with,
// its tests and its errors file (zz_generated_errors_<part>) included.
func unitOf(name string) string {
	if dir := path.Dir(name); dir != "." {
		first, _, _ := strings.Cut(dir, "/")
		return first
	}
	stem := strings.TrimSuffix(strings.TrimPrefix(path.Base(name), "zz_generated_"), ".go"+templateSuffix)
	stem = strings.TrimSuffix(strings.TrimSuffix(stem, "_internal_test"), "_test")
	if part, ok := strings.CutPrefix(stem, "errors_"); ok {
		return part
	}
	first, _, _ := strings.Cut(stem, "_")
	return first
}

// selectUnits keeps the parts of the support package the repository uses
// (the brownfield fix of M7): a part is kept when a Go file outside the
// support path imports it or names one of its symbols, when one of its files
// is already in the repository, or when a kept part needs it. A repository
// that uses no part yet, as on golaw new support, gets the whole package.
func selectUnits(in genkind.Input, all []rendered) []rendered {
	if in.Snapshot == nil {
		return all
	}
	root := in.Module + "/" + in.Policy.Support
	symbols := rootSymbols(all, in.Policy.Support)
	used := map[string]bool{}
	for _, f := range in.Snapshot.Files() {
		if inSupport(f.Path, in.Policy.Support) {
			used[unitOf(strings.TrimPrefix(f.Path, in.Policy.Support+"/")+templateSuffix)] = true
			continue
		}
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}
		content, err := in.Snapshot.Read(f.Path)
		if err != nil || !bytes.Contains(content, []byte(`"`+root)) {
			continue
		}
		references(content, root, symbols, used)
	}
	if len(used) == 0 {
		return all
	}
	return closure(all, in.Policy.Support, root, symbols, used)
}

func inSupport(file, support string) bool {
	return strings.HasPrefix(file, support+"/")
}

// rootSymbols maps each top level name of the root package to its part.
func rootSymbols(all []rendered, support string) map[string]string {
	symbols := map[string]string{}
	for _, r := range all {
		if path.Dir(r.file.Path) != support || strings.HasSuffix(r.file.Path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), r.file.Path, r.file.Content, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		for _, name := range topLevel(file) {
			symbols[name] = r.unit
		}
	}
	return symbols
}

func topLevel(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				names = append(names, d.Name.Name)
			}
		case *ast.GenDecl:
			names = append(names, specNames(d.Specs)...)
		}
	}
	return names
}

func specNames(specs []ast.Spec) []string {
	var names []string
	for _, spec := range specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			names = append(names, s.Name.Name)
		case *ast.ValueSpec:
			for _, n := range s.Names {
				names = append(names, n.Name)
			}
		}
	}
	return names
}

// references marks the parts one Go file uses: a subpackage by its import,
// a root part by a selector on the name the root package is imported as.
func references(content []byte, root string, symbols map[string]string, used map[string]bool) {
	file, err := parser.ParseFile(token.NewFileSet(), "", content, parser.SkipObjectResolution)
	if err != nil {
		return
	}
	local := imports(file, root, used)
	if local == "" {
		return
	}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == local {
			if unit, ok := symbols[sel.Sel.Name]; ok {
				used[unit] = true
			}
		}
		return true
	})
}

// imports marks each support subpackage the file imports and answers the
// name the root package is imported as, or nothing when it is not.
func imports(file *ast.File, root string, used map[string]bool) string {
	local := ""
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if sub, ok := strings.CutPrefix(imported, root+"/"); ok {
			first, _, _ := strings.Cut(sub, "/")
			used[first] = true
			continue
		}
		if imported != root {
			continue
		}
		local = path.Base(root)
		if spec.Name != nil {
			local = spec.Name.Name
		}
	}
	return local
}

// closure adds the parts the kept parts need, until none is added, and
// answers the kept files in the order they were rendered.
func closure(all []rendered, support, root string, symbols map[string]string, used map[string]bool) []rendered {
	for grown := true; grown; {
		grown = false
		for _, r := range all {
			if !used[r.unit] {
				continue
			}
			before := len(used)
			needs(r.file, support, root, symbols, used)
			grown = grown || len(used) > before
		}
	}
	var kept []rendered
	for _, r := range all {
		if used[r.unit] {
			kept = append(kept, r)
		}
	}
	return kept
}

// needs marks the parts one support file uses: a root file names its own
// package's symbols bare, any other file reaches the root through imports.
func needs(f genkind.File, support, root string, symbols map[string]string, used map[string]bool) {
	if path.Dir(f.Path) != support {
		references(f.Content, root, symbols, used)
		return
	}
	file, err := parser.ParseFile(token.NewFileSet(), f.Path, f.Content, parser.SkipObjectResolution)
	if err != nil {
		return
	}
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if unit, ok := symbols[id.Name]; ok {
				used[unit] = true
			}
		}
		return true
	})
	references(f.Content, root, symbols, used)
}
