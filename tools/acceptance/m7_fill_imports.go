package main

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// m7AddImports adds the imports content lacks to its first import
// declaration, or a new block after the package clause, and formats the
// result; nothing to add answers content as it is.
func m7AddImports(content []byte, imports []m7Import) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parse the imports: %w", err)
	}
	lines := m7MissingImports(file, imports)
	if len(lines) == 0 {
		return content, nil
	}
	at := func(pos token.Pos) int { return fset.Position(pos).Offset }
	added := strings.Join(lines, "\n")
	var edit m7Edit
	switch decl := m7FirstImport(file); {
	case decl == nil:
		end := at(file.Name.End())
		edit = m7Edit{from: end, to: end, text: "\n\nimport (\n" + added + "\n)"}
	case decl.Lparen.IsValid():
		edit = m7Edit{from: at(decl.Rparen), to: at(decl.Rparen), text: added + "\n"}
	default:
		spec := string(content[at(decl.Specs[0].Pos()):at(decl.Specs[0].End())])
		edit = m7Edit{from: at(decl.Pos()), to: at(decl.End()), text: "import (\n" + spec + "\n" + added + "\n)"}
	}
	out, err := format.Source(m7Replace(content, []m7Edit{edit}))
	if err != nil {
		return nil, fmt.Errorf("format the imports: %w", err)
	}
	return out, nil
}

// m7MissingImports renders the imports file lacks, each once.
func m7MissingImports(file *ast.File, imports []m7Import) []string {
	have := make(map[string]bool)
	for _, spec := range file.Imports {
		have[m7ImportPath(spec)] = true
	}
	var lines []string
	for _, imp := range imports {
		if have[imp.Path] {
			continue
		}
		have[imp.Path] = true
		line := strconv.Quote(imp.Path)
		if imp.Name != "" {
			line = imp.Name + " " + line
		}
		lines = append(lines, "\t"+line)
	}
	return lines
}

// m7FirstImport answers the first import declaration of file, if any.
func m7FirstImport(file *ast.File) *ast.GenDecl {
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			return gen
		}
	}
	return nil
}

// m7PruneImports drops the imports only the hole used, as the person's
// editor would, and formats the file; a fill replaces the hole call, so
// the support package or fmt of the stub may be left unused.
func m7PruneImports(content []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse the filled file: %w", err)
	}
	used := m7SelectorNames(file)
	at := func(pos token.Pos) int { return fset.Position(pos).Offset }
	var edits []m7Edit
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			imp, isImport := spec.(*ast.ImportSpec)
			switch {
			case !isImport || m7Kept(imp, used):
			case gen.Lparen.IsValid():
				edits = append(edits, m7Edit{from: at(imp.Pos()), to: at(imp.End())})
			default:
				edits = append(edits, m7Edit{from: at(gen.Pos()), to: at(gen.End())})
			}
		}
	}
	formatted, err := format.Source(m7Replace(content, edits))
	if err != nil {
		return nil, fmt.Errorf("format the filled file: %w", err)
	}
	return formatted, nil
}

// m7SelectorNames are the names file qualifies a selector with, the only
// way a file uses a named import.
func m7SelectorNames(file *ast.File) map[string]bool {
	used := make(map[string]bool)
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if x, isIdent := sel.X.(*ast.Ident); isIdent {
				used[x.Name] = true
			}
		}
		return true
	})
	return used
}

// m7Kept answers whether the file still uses imp; a blank or dot import
// is used without a selector, so it is always kept.
func m7Kept(imp *ast.ImportSpec, used map[string]bool) bool {
	if imp.Name != nil {
		return imp.Name.Name == "_" || imp.Name.Name == "." || used[imp.Name.Name]
	}
	path := m7ImportPath(imp)
	return used[path[strings.LastIndex(path, "/")+1:]]
}

// m7ImportPath answers the path of imp; the parser accepted the file, so
// the literal is a quoted or raw string and only its quotes go.
func m7ImportPath(imp *ast.ImportSpec) string {
	return strings.Trim(imp.Path.Value, "\"`")
}
