package text

import (
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// bareNolint is assembled from halves so that this file carries no bare
// suppression of its own. It matches a directive without a linter (alone,
// before a reason, or before a word, which golangci-lint reads as a
// suppression of every linter) and a linter list without a reason or with
// an empty one.
var bareNolint = regexp.MustCompile(`//\s*nol` + `int(?:\s*$|\s+[^:\s]|\s*//|:[\w,-]+\s*(?://\s*)?$)`)

// BareNolint is text.bare-nolint: every lint suppression names its linter
// and says why.
func BareNolint() model.Check {
	return lineRule{spec: spec("text.bare-nolint"), accept: goFile, terms: []term{
		{pattern: bareNolint, code: "bare-nolint", message: "lint suppression without its linter or a justification"},
	}}
}
