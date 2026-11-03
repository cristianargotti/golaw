package genmap

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"unicode"
)

// maxConsts bounds the constants listed for one type; an enum longer than
// this is named by its first ones and a count.
const maxConsts = 16

// constsOf lists the constants of a type in its package: "consts
// Proposed=\"PROPOSTO\", Approved", typed by a declaration, a conversion
// or the iota line they continue.
func constsOf(p pkgSrc, typ string) string {
	var names []string
	for _, s := range p.srcs {
		for _, decl := range s.file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.CONST {
				names = append(names, blockConsts(gen, typ)...)
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	if len(names) > maxConsts {
		names = append(names[:maxConsts], "and "+strconv.Itoa(len(names)-maxConsts)+" more")
	}
	return "consts " + strings.Join(names, ", ")
}

// blockConsts answers the constants of one const block that have the type.
func blockConsts(gen *ast.GenDecl, typ string) []string {
	var out []string
	last := ""
	for _, spec := range gen.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		last = specType(vs, last)
		if last != typ {
			continue
		}
		for i, n := range vs.Names {
			out = append(out, constText(n.Name, vs, i))
		}
	}
	return out
}

// specType is the type of a const spec: its declared type, the type of a
// conversion it holds, the previous spec's for a bare iota line, else none.
func specType(vs *ast.ValueSpec, last string) string {
	switch {
	case vs.Type != nil:
		return typeName(vs.Type)
	case len(vs.Values) == 0:
		return last
	}
	if call, ok := vs.Values[0].(*ast.CallExpr); ok && len(call.Args) == 1 {
		return typeName(call.Fun)
	}
	return ""
}

// constText is a constant with its literal value when it has one.
func constText(name string, vs *ast.ValueSpec, i int) string {
	if i >= len(vs.Values) {
		return name
	}
	v := vs.Values[i]
	if call, ok := v.(*ast.CallExpr); ok && len(call.Args) == 1 {
		v = call.Args[0]
	}
	if lit, ok := v.(*ast.BasicLit); ok {
		return name + "=" + lit.Value
	}
	return name
}

// addWords keeps the lower case words of a name, "PlanLedger" giving plan
// and ledger, so a query file named after one of them is the area's.
func (r *areaReader) addWords(name string) {
	for _, w := range splitWords(name) {
		r.words[w] = true
	}
}

// splitWords splits a camel case name into lower case words.
func splitWords(name string) []string {
	var out []string
	start := 0
	runes := []rune(name)
	for i := 1; i <= len(runes); i++ {
		if i == len(runes) || unicode.IsUpper(runes[i]) && !unicode.IsUpper(runes[i-1]) {
			out = append(out, strings.ToLower(string(runes[start:i])))
			start = i
		}
	}
	return out
}

// namesWord reports whether a file stem, plural or not, is a word of the
// area: "plans" and "plan" both name the word plan.
func (r *areaReader) namesWord(stem string) bool {
	stem = strings.ToLower(stem)
	return r.words[stem] || r.words[strings.TrimSuffix(stem, "s")]
}
