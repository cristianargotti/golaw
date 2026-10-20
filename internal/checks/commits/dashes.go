package commits

import (
	"strings"
	"unicode/utf8"

	"github.com/cristianargotti/golaw/internal/model"
)

// The two typographic dashes by code point, so this file carries neither.
const (
	emDashRune = 0x2014
	enDashRune = 0x2013
)

// NoDashes is commit.no-dashes: no em dash or en dash in the message git
// records, subject or body; a comment line git strips is not recorded and
// never a finding.
func NoDashes() model.Check {
	return messageRule{spec: spec("commit.no-dashes"), judge: judgeDashes}
}

func judgeDashes(message, _ string) []model.Finding {
	var out []model.Finding
	for i, line := range lines(message) {
		if column := runeColumn(line, emDashRune); column > 0 {
			out = append(out, deny("em-dash", i+1, column, "em dash (U+2014) in the commit message; use a comma, a colon or a hyphen"))
		}
		if column := runeColumn(line, enDashRune); column > 0 {
			out = append(out, deny("en-dash", i+1, column, "en dash (U+2013) in the commit message; use a comma, a colon or a hyphen"))
		}
	}
	return out
}

// runeColumn answers the 1-based rune column of the first r of the line, 0
// when the line holds none.
func runeColumn(line string, r rune) int {
	i := strings.IndexRune(line, r)
	if i < 0 {
		return 0
	}
	return utf8.RuneCountInString(line[:i]) + 1
}
