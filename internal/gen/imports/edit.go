package imports

import (
	"bytes"
	"cmp"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// change replaces content[start:end] with text; an insertion has start
// equal to end.
type change struct {
	start, end int
	text       string
}

// edit works on the bytes rather than reprinting the tree, so every
// comment and blank line outside the import lines stays where it was.
func edit(fset *token.FileSet, file *ast.File, content []byte, p plan) []byte {
	tf := fset.File(file.Pos())
	gone := map[ast.Spec]bool{}
	for _, spec := range p.remove {
		gone[spec] = true
	}
	var changes []change
	var decls []*ast.GenDecl
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			decls = append(decls, gen)
			changes = append(changes, removals(tf, content, gen, gone)...)
		}
	}
	changes = append(changes, insertions(tf, content, file, decls, gone, p.add)...)
	slices.SortStableFunc(changes, func(a, b change) int {
		return cmp.Or(cmp.Compare(b.start, a.start), cmp.Compare(b.end-b.start, a.end-a.start))
	})
	out := content
	for _, c := range changes {
		out = slices.Concat(out[:c.start], []byte(c.text), out[c.end:])
	}
	return out
}

// removals takes a whole declaration when all of its specs go, so no empty
// import block is left behind.
func removals(tf *token.File, content []byte, decl *ast.GenDecl, gone map[ast.Spec]bool) []change {
	var out []change
	for _, s := range decl.Specs {
		if gone[s] {
			out = append(out, change{start: lineStart(content, tf.Offset(s.Pos())), end: lineEnd(content, tf.Offset(s.End()))})
		}
	}
	if len(out) > 0 && len(out) == len(decl.Specs) {
		return []change{{start: lineStart(content, tf.Offset(decl.Pos())), end: lineEnd(content, tf.Offset(decl.End()))}}
	}
	return out
}

// insertions puts the standard paths at the head of the first block that
// spans lines and the module paths after its last spec as their own group,
// the grouping gci and goimports keep.
func insertions(tf *token.File, content []byte, file *ast.File, decls []*ast.GenDecl, gone map[ast.Spec]bool, add []string) []change {
	if len(add) == 0 {
		return nil
	}
	var std, mod []string
	for _, importPath := range add {
		if first, _, _ := strings.Cut(importPath, "/"); strings.Contains(first, ".") {
			mod = append(mod, importPath)
		} else {
			std = append(std, importPath)
		}
	}
	if decl := blockOf(tf, decls, gone); decl != nil {
		return intoBlock(tf, content, decl, std, mod)
	}
	block := "import (\n" + groups(std, mod) + ")\n"
	if len(decls) == 0 {
		return []change{at(lineEnd(content, tf.Offset(file.Name.End())), "\n"+block)}
	}
	return []change{at(lineEnd(content, tf.Offset(decls[len(decls)-1].End())), block)}
}

func intoBlock(tf *token.File, content []byte, decl *ast.GenDecl, std, mod []string) []change {
	if len(decl.Specs) == 0 {
		start := lineEnd(content, tf.Offset(decl.Lparen))
		return []change{at(start, groups(std, mod))}
	}
	var out []change
	if len(std) > 0 {
		out = append(out, at(lineStart(content, tf.Offset(decl.Specs[0].Pos())), specLines(std)))
	}
	if len(mod) > 0 {
		out = append(out, at(lineEnd(content, tf.Offset(decl.Specs[len(decl.Specs)-1].End())), "\n"+specLines(mod)))
	}
	return out
}

// blockOf answers the first parenthesized declaration that survives the
// removals and keeps its parentheses on lines of their own, the only shape
// a line insertion cannot break.
func blockOf(tf *token.File, decls []*ast.GenDecl, gone map[ast.Spec]bool) *ast.GenDecl {
	for _, decl := range decls {
		allGone := len(decl.Specs) > 0 && !slices.ContainsFunc(decl.Specs, func(s ast.Spec) bool { return !gone[s] })
		if !decl.Lparen.IsValid() || allGone {
			continue
		}
		first, last := decl.Rparen, decl.Lparen
		if len(decl.Specs) > 0 {
			first, last = decl.Specs[0].Pos(), decl.Specs[len(decl.Specs)-1].End()
		}
		if tf.Line(decl.Lparen) < tf.Line(first) && tf.Line(last) < tf.Line(decl.Rparen) {
			return decl
		}
	}
	return nil
}

func at(offset int, text string) change { return change{start: offset, end: offset, text: text} }

// groups puts a blank line before the module paths: their own group.
func groups(std, mod []string) string {
	if len(std) > 0 && len(mod) > 0 {
		return specLines(std) + "\n" + specLines(mod)
	}
	return specLines(std) + specLines(mod)
}

func specLines(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return "\t\"" + strings.Join(paths, "\"\n\t\"") + "\"\n"
}

func lineStart(content []byte, offset int) int {
	return bytes.LastIndexByte(content[:offset], '\n') + 1
}

// lineEnd answers the offset after the newline that ends the line, or the
// length of a file whose last line has none.
func lineEnd(content []byte, offset int) int {
	if i := bytes.IndexByte(content[offset:], '\n'); i >= 0 {
		return offset + i + 1
	}
	return len(content)
}
