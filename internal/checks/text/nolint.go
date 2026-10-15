package text

import (
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// bareNolint is assembled from halves so that this file carries no bare
// suppression of its own.
var bareNolint = regexp.MustCompile(`//\s*nol` + `int(?::[\w,-]+)?\s*$`)

// BareNolint is text.bare-nolint: every lint suppression names its linter
// and says why.
func BareNolint() model.Check {
	return lineRule{spec: spec("text.bare-nolint"), accept: goFile, terms: []term{
		{pattern: bareNolint, code: "bare-nolint", message: "lint suppression without its linter or a justification"},
	}}
}
