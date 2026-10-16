package goast

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"slices"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	codeLiteralRepeat = "literal-repeat"
	literalThreshold  = 3
	literalMinLength  = 5
)

// wordOnly matches a literal made of word characters only, which Sonar
// never counts as a duplicated literal.
var wordOnly = regexp.MustCompile(`^\w+$`)

// SonarLiteralRepeat is goast.sonar-literal-repeat (Sonar rule S1192): an
// interpreted string literal longer than five characters that is not a
// bare word and appears three or more times in one file is a finding at
// its first occurrence. A struct tag never counts, whatever its quotes:
// the language takes a tag only as a literal, so it cannot be named once.
func SonarLiteralRepeat() model.Check {
	return check{spec: spec("sonar-literal-repeat"), judge: judgeLiteralRepeat}
}

// literalCount is the first position and the count of one literal.
type literalCount struct {
	first token.Pos
	count int
}

func judgeLiteralRepeat(t *target, _ *state) ([]model.Finding, error) {
	file, _, err := t.parsed()
	if err != nil {
		return nil, err
	}
	counts := map[string]*literalCount{}
	tags := map[*ast.BasicLit]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		if field, ok := n.(*ast.Field); ok && field.Tag != nil {
			tags[field.Tag] = true
		}
		lit, ok := n.(*ast.BasicLit)
		if !ok || tags[lit] || lit.Kind != token.STRING || lit.Value[0] != '"' {
			return true
		}
		value, ok := stringLiteral(lit)
		if !ok || utf8.RuneCountInString(value) <= literalMinLength || wordOnly.MatchString(value) {
			return true
		}
		if counts[value] == nil {
			counts[value] = &literalCount{first: lit.Pos()}
		}
		counts[value].count++
		return true
	})
	return repeatedLiterals(t, counts), nil
}

// repeatedLiterals turns the counts at the threshold into findings in
// source order. The message names the count and the finding the place,
// never the value: a repeated literal can be a credential, and a report
// never carries a secret value.
func repeatedLiterals(t *target, counts map[string]*literalCount) []model.Finding {
	var out []model.Finding
	for _, c := range counts {
		if c.count >= literalThreshold {
			message := fmt.Sprintf("a string literal appears %d times in the file: name it once", c.count)
			out = append(out, t.finding(codeLiteralRepeat, c.first, message))
		}
	}
	slices.SortFunc(out, func(a, b model.Finding) int { return a.Line - b.Line })
	return out
}
