package text

import (
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// The two typographic dashes by code point, so this file carries neither.
const (
	emDashRune = 0x2014
	enDashRune = 0x2013
)

var (
	emDash = regexp.MustCompile(string(rune(emDashRune)))
	enDash = regexp.MustCompile(string(rune(enDashRune)))
)

// Dashes is text.dashes: no em dash or en dash in any text file, source and
// documentation included.
func Dashes() model.Check {
	return lineRule{spec: spec("text.dashes"), accept: anyText, terms: []term{
		{pattern: emDash, code: "em-dash", message: "em dash (U+2014) in text; use a comma, a colon or a hyphen"},
		{pattern: enDash, code: "en-dash", message: "en dash (U+2013) in text; use a comma, a colon or a hyphen"},
	}}
}
