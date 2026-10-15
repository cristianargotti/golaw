package text

import (
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The nilnil pattern is assembled from halves so that this file carries no
// suppression of its own.
var nolintNilnil = regexp.MustCompile(`//\s*nol` + `int:[^/\n]*\bnilnil\b`)

// goSource keeps hand-written Go files that are not tests.
func goSource(f model.File) bool {
	return goFile(f) && f.Class != model.ClassTest && !strings.HasSuffix(f.Path, "_test.go")
}

// NolintNilnil is text.nolint-nilnil: a nil, nil return is never hidden
// behind a suppression in application code.
func NolintNilnil() model.Check {
	return lineRule{spec: spec("text.nolint-nilnil"), accept: goSource, terms: []term{
		{pattern: nolintNilnil, code: "nolint-nilnil", message: "nil, nil return hidden behind a suppression; answer a value or a typed sentinel"},
	}}
}
