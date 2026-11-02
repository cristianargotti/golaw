package holes

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"slices"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// stubPackage wraps one declaration so it parses and formats alone.
const stubPackage = "package p\n\n"

// refused wraps ErrAppend with the owned file and the reason, one spelling
// for every refusal so a caller reads them alike.
const refused = "%w: %s: %w"

// Names answers the top level declaration keys of a Go file, sorted and
// unique: a method is "Recv.Name" so two receivers never collide.
func Names(content []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "", content, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("read the declarations: %w", err)
	}
	var out []string
	for _, decl := range file.Decls {
		out = append(out, keys(decl)...)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// keys answers the names one declaration adds to the package scope; the
// blank name declares nothing, so it never refuses a stub.
func keys(decl ast.Decl) []string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Recv == nil || len(d.Recv.List) == 0 {
			return []string{d.Name.Name}
		}
		return []string{baseType(d.Recv.List[0].Type) + "." + d.Name.Name}
	case *ast.GenDecl:
		var out []string
		for _, spec := range d.Specs {
			out = append(out, specNames(spec)...)
		}
		return out
	}
	return nil
}

// specNames answers the names of one type, variable or constant spec.
func specNames(spec ast.Spec) []string {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		return []string{s.Name.Name}
	case *ast.ValueSpec:
		var out []string
		for _, n := range s.Names {
			if n.Name != "_" {
				out = append(out, n.Name)
			}
		}
		return out
	}
	return nil
}

// baseType is the receiver's type name, its star, parentheses and type
// arguments dropped.
func baseType(expr ast.Expr) string {
	name := receiver(expr)
	if name != "" && name[0] == '*' {
		return name[1:]
	}
	return name
}

// Append adds each declaration of add at the end of an owned file and its
// missing imports to the import block; every other byte stays as written,
// because owned code is the agent's (M7.4.2 rule 5).
func Append(content []byte, add genkind.Append) ([]byte, error) {
	declared, err := Names(content)
	if err != nil {
		return nil, fmt.Errorf(refused, ErrAppend, add.Path, err)
	}
	taken := map[string]bool{}
	for _, name := range declared {
		taken[name] = true
	}
	var decls [][]byte
	for _, raw := range add.Decls {
		decl, stubErr := stub(raw, add.Path, taken)
		if stubErr != nil {
			return nil, stubErr
		}
		decls = append(decls, decl)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, add.Path, content, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf(refused, ErrAppend, add.Path, err)
	}
	out := withImports(content, file, fset, add.Imports)
	out = append(bytes.TrimRight(out, "\n"), '\n')
	for _, decl := range decls {
		out = append(append(out, '\n'), decl...)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), add.Path, out, parser.SkipObjectResolution); err != nil {
		return nil, fmt.Errorf("%w: %s: the result does not parse: %w", ErrAppend, add.Path, err)
	}
	return out, nil
}

// stub checks one declaration alone and answers it formatted; a name the
// file or an earlier stub declares is refused and recorded in taken.
func stub(raw []byte, path string, taken map[string]bool) ([]byte, error) {
	wrapped := append([]byte(stubPackage), raw...)
	file, err := parser.ParseFile(token.NewFileSet(), path, wrapped, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf(refused, ErrAppend, path, err)
	}
	if len(file.Decls) != 1 {
		return nil, fmt.Errorf("%w: %s: %d declarations in one stub", ErrAppend, path, len(file.Decls))
	}
	for _, key := range keys(file.Decls[0]) {
		if taken[key] {
			return nil, fmt.Errorf("%w: %s in %s", ErrDeclared, key, path)
		}
		taken[key] = true
	}
	formatted, err := format.Source(wrapped)
	if err != nil {
		return nil, fmt.Errorf(refused, ErrAppend, path, err)
	}
	return bytes.TrimPrefix(formatted, []byte(stubPackage)), nil
}

// lineStart is the offset of the first byte of the line holding offset.
func lineStart(content []byte, offset int) int {
	for offset > 0 && content[offset-1] != '\n' {
		offset--
	}
	return offset
}

// lineEnd is the offset just past the newline of the line holding offset.
func lineEnd(content []byte, offset int) int {
	for offset < len(content) && content[offset] != '\n' {
		offset++
	}
	if offset < len(content) {
		offset++
	}
	return offset
}
