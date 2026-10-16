package text

import (
	"context"
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// term is one pattern a line rule reports, with its stable code.
type term struct {
	pattern *regexp.Regexp
	code    string
	message string
}

// lineRule reports every line of its targets that matches one of its terms.
type lineRule struct {
	spec   model.Spec
	accept keep
	terms  []term
}

func (r lineRule) Spec() model.Spec { return r.spec }

// Run walks the targets and reports the first match of every term per line.
func (r lineRule) Run(_ context.Context, in model.Input) (model.Result, error) {
	var findings []model.Finding
	read, err := walk(in, r.accept, func(path string, number int, line string) {
		findings = append(findings, matches(r.terms, path, number, line)...)
	})
	return result(read, err, findings), nil
}

// matches reports the terms a line carries, one finding per term.
func matches(terms []term, path string, number int, line string) []model.Finding {
	var out []model.Finding
	for _, t := range terms {
		if loc := t.pattern.FindStringIndex(line); loc != nil {
			out = append(out, finding(path, number, column(line, loc[0]), t.code, t.message))
		}
	}
	return out
}
