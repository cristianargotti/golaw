package holes

import (
	"cmp"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// insertion is text placed at one byte offset of the owned file.
type insertion struct {
	at   int
	text string
}

// withImports adds the imports whose path the file lacks, in path order;
// the bytes of every existing spec and comment stay as they are.
func withImports(content []byte, file *ast.File, fset *token.FileSet, add []genkind.Import) []byte {
	have := map[string]bool{}
	for _, spec := range file.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		have[p] = true
	}
	var missing []genkind.Import
	for _, imp := range add {
		if !have[imp.Path] {
			have[imp.Path] = true
			missing = append(missing, imp)
		}
	}
	if len(missing) == 0 {
		return content
	}
	slices.SortFunc(missing, func(a, b genkind.Import) int { return cmp.Compare(a.Path, b.Path) })
	tf := fset.File(file.Pos())
	decl := firstImport(file)
	switch {
	case decl == nil:
		block := "\n\nimport (\n" + groupedLines(nil, missing) + ")"
		return insertAll(content, []insertion{{at: tf.Offset(file.Name.End()), text: block}})
	case oneLine(tf, decl):
		start, end := tf.Offset(decl.Pos()), tf.Offset(decl.End())
		block := "import (\n" + groupedLines(writtenSpecs(content, tf, decl), missing) + ")"
		return slices.Concat(content[:start], []byte(block), content[end:])
	}
	return insertAll(content, insertAt(content, tf, decl, missing))
}

// firstImport is the first import declaration of the file, nil without one.
func firstImport(file *ast.File) *ast.GenDecl {
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			return gen
		}
	}
	return nil
}

// groupOf is the group a path sorts into: the standard library, which has
// no dot in its first element, or the first element itself.
func groupOf(importPath string) string {
	first, _, _ := strings.Cut(importPath, "/")
	if !strings.Contains(first, ".") {
		return ""
	}
	return first
}

// specLine is one import line of a block without its newline.
func specLine(imp genkind.Import) string {
	if imp.Name != "" {
		return "\t" + imp.Name + " " + strconv.Quote(imp.Path)
	}
	return "\t" + strconv.Quote(imp.Path)
}

// oneLine reports a declaration without parentheses, or one whose closing
// parenthesis shares a line with a spec, where no line can be inserted.
func oneLine(tf *token.File, decl *ast.GenDecl) bool {
	if !decl.Lparen.IsValid() || tf.Line(decl.Lparen) == tf.Line(decl.Rparen) {
		return true
	}
	return len(decl.Specs) > 0 && tf.Line(decl.Specs[len(decl.Specs)-1].End()) == tf.Line(decl.Rparen)
}

// importLine is one line of a rebuilt block, sorted by group then path.
type importLine struct{ group, path, text string }

// writtenSpecs keeps each spec's bytes as written, its name included.
func writtenSpecs(content []byte, tf *token.File, decl *ast.GenDecl) []importLine {
	out := make([]importLine, 0, len(decl.Specs))
	for _, spec := range decl.Specs {
		is, ok := spec.(*ast.ImportSpec)
		if !ok {
			continue
		}
		p, err := strconv.Unquote(is.Path.Value)
		if err != nil {
			continue
		}
		text := "\t" + string(content[tf.Offset(is.Pos()):tf.Offset(is.End())])
		out = append(out, importLine{groupOf(p), p, text})
	}
	return out
}

// groupedLines writes the specs as written followed by the new ones, the
// standard library first and then one group per first path element.
func groupedLines(lines []importLine, add []genkind.Import) string {
	for _, imp := range add {
		lines = append(lines, importLine{groupOf(imp.Path), imp.Path, specLine(imp)})
	}
	slices.SortStableFunc(lines, func(a, b importLine) int {
		return cmp.Or(cmp.Compare(a.group, b.group), cmp.Compare(a.path, b.path))
	})
	out := make([]string, 0, 2*len(lines))
	for i, l := range lines {
		if i > 0 && lines[i-1].group != l.group {
			out = append(out, "")
		}
		out = append(out, l.text)
	}
	return strings.Join(out, "\n") + "\n"
}

// insertAt places each new spec in its group of a parenthesized block:
// before the first spec of the group that sorts after it, else after the
// group's last spec; a group the block lacks opens at its end.
func insertAt(content []byte, tf *token.File, decl *ast.GenDecl, add []genkind.Import) []insertion {
	var out []insertion
	opened := map[string]bool{}
	closing := lineStart(content, tf.Offset(decl.Rparen))
	for _, imp := range add {
		group := groupOf(imp.Path)
		at, found := placeIn(content, tf, decl, imp.Path, group)
		switch {
		case found:
			out = append(out, insertion{at: at, text: specLine(imp) + "\n"})
		case opened[group]:
			out = append(out, insertion{at: closing, text: specLine(imp) + "\n"})
		default:
			opened[group] = true
			out = append(out, insertion{at: closing, text: "\n" + specLine(imp) + "\n"})
		}
	}
	return out
}

// placeIn answers the offset a path of group goes to among the block's
// specs, and false when no spec of the block belongs to the group.
func placeIn(content []byte, tf *token.File, decl *ast.GenDecl, importPath, group string) (int, bool) {
	last := -1
	for _, spec := range decl.Specs {
		is, ok := spec.(*ast.ImportSpec)
		if !ok {
			continue
		}
		p, err := strconv.Unquote(is.Path.Value)
		if err != nil || groupOf(p) != group {
			continue
		}
		if p > importPath {
			return lineStart(content, tf.Offset(is.Pos())), true
		}
		last = lineEnd(content, tf.Offset(is.End()))
	}
	return last, last >= 0
}

// insertAll applies insertions in offset order; two at one offset keep
// their order, so the new specs of one place stay sorted.
func insertAll(content []byte, ins []insertion) []byte {
	slices.SortStableFunc(ins, func(a, b insertion) int { return cmp.Compare(a.at, b.at) })
	out := make([]byte, 0, len(content))
	prev := 0
	for _, in := range ins {
		out = append(append(out, content[prev:in.at]...), in.text...)
		prev = in.at
	}
	return append(out, content[prev:]...)
}
