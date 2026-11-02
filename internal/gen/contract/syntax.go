package contract

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// parse reads one contract file with its comments, because the doc of a port
// and of a method and the route comment of an operation are part of what a
// kind renders; object resolution is skipped since no reader needs it.
func parse(path string, content []byte) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return file, fset, nil
}

// source answers the bytes of a node as the contract writes them, so a type
// keeps its own spelling and a template writes it back unchanged.
func source(fset *token.FileSet, content []byte, node ast.Node) string {
	return string(content[fset.Position(node.Pos()).Offset:fset.Position(node.End()).Offset])
}

// params turns a parameter or result list into the contract model, one Param
// per name so "dst, src string" stays two parameters of one type.
func params(fset *token.FileSet, content []byte, list *ast.FieldList) []genkind.Param {
	if list == nil || len(list.List) == 0 {
		return nil
	}
	var out []genkind.Param
	for _, field := range list.List {
		typ := source(fset, content, field.Type)
		if len(field.Names) == 0 {
			out = append(out, genkind.Param{Type: typ})
			continue
		}
		for _, name := range field.Names {
			out = append(out, genkind.Param{Name: name.Name, Type: typ})
		}
	}
	return out
}

// doc answers a comment group's text without the slashes and the trailing
// newline, empty when the declaration has no doc.
func doc(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return strings.TrimSpace(group.Text())
}

// imports answers the file's imports in source order, the explicit name kept
// so generated code names a renamed import the way the contract does.
func imports(file *ast.File) []genkind.Import {
	if len(file.Imports) == 0 {
		return nil
	}
	out := make([]genkind.Import, 0, len(file.Imports))
	for _, spec := range file.Imports {
		imp := genkind.Import{}
		if spec.Name != nil {
			imp.Name = spec.Name.Name
		}
		imp.Path = spec.Path.Value
		if unquoted, err := strconv.Unquote(spec.Path.Value); err == nil { // fallback: the parser accepted the literal, so the raw spelling still names the import
			imp.Path = unquoted
		}
		out = append(out, imp)
	}
	return out
}
