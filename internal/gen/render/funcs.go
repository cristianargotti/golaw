package render

import (
	"strconv"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
)

// errorType is the result type a hole answers its error through.
const errorType = "error"

// Funcs are the functions every kind template may call; kebab is the hole
// model's own, so a template's hole id and the finder's are one spelling.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"kebab":      holes.Kebab,
		"lowerFirst": LowerFirst,
		"qualify":    Qualify,
		"params":     Params,
		"names":      Names,
		"results":    Results,
		"zero":       Zero,
		"returnZero": ReturnZero,
	}
}

// LowerFirst lowers the first rune alone: "ReadPlan" is "readPlan".
func LowerFirst(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 {
		return s
	}
	return string(unicode.ToLower(r)) + s[size:]
}

// Params spells a parameter list as Go source; an unnamed parameter is the
// blank name, because a list mixing named and unnamed ones does not parse.
func Params(ps []genkind.Param) string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		name := p.Name
		if name == "" {
			name = "_"
		}
		out = append(out, name+" "+p.Type)
	}
	return strings.Join(out, ", ")
}

// Names joins the parameter names a call can pass on.
func Names(ps []genkind.Param) string {
	var out []string
	for _, p := range ps {
		if p.Name != "" && p.Name != "_" {
			out = append(out, p.Name)
		}
	}
	return strings.Join(out, ", ")
}

// Results spells a result list with its names dropped, because the lint
// asset turns nonamedreturns on.
func Results(ps []genkind.Param) string {
	switch len(ps) {
	case 0:
		return ""
	case 1:
		return ps[0].Type
	}
	types := make([]string, 0, len(ps))
	for _, p := range ps {
		types = append(types, p.Type)
	}
	return "(" + strings.Join(types, ", ") + ")"
}

// ReturnZero spells the return of a hole: the zero of each result, a
// variable for a type the syntax cannot spell a zero for, and errExpr for
// the error result. The variables skip every parameter name.
func ReturnZero(results, params []genkind.Param, errExpr string) string {
	if len(results) == 0 {
		return "return"
	}
	taken := map[string]bool{}
	for _, p := range params {
		taken[p.Name] = true
	}
	var lines, values []string
	next := 1
	for _, r := range results {
		if r.Type == errorType {
			values = append(values, errExpr)
			continue
		}
		if zero := Zero(r.Type); zero != "" {
			values = append(values, zero)
			continue
		}
		name := zeroName(next)
		for ; taken[name]; name = zeroName(next) {
			next++
		}
		taken[name] = true
		lines = append(lines, "var "+name+" "+r.Type)
		values = append(values, name)
	}
	return strings.Join(append(lines, "return "+strings.Join(values, ", ")), "\n")
}

// zeroName is "zero" for the first variable and "zero<n>" for the others.
func zeroName(n int) string {
	if n == 1 {
		return "zero"
	}
	return "zero" + strconv.Itoa(n)
}
