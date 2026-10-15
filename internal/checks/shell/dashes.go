package shell

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The two typographic dashes by code point, so this file carries neither.
const (
	emDashRune = 0x2014
	enDashRune = 0x2013
)

// Dashes is shell.dashes: no em dash or en dash in a commit message given on
// the command line.
func Dashes() model.Check {
	return commandRule{spec: spec("shell.dashes"), judge: judgeDashes}
}

func judgeDashes(_ model.Input, segs []segment) ([]model.Finding, error) {
	for _, call := range gitCalls(segs) {
		if !isCommit(call) {
			continue
		}
		switch message := messageOf(call); {
		case strings.ContainsRune(message, emDashRune):
			return []model.Finding{deny("em-dash", "em dash (U+2014) in the commit message; use a comma, a colon or a hyphen")}, nil
		case strings.ContainsRune(message, enDashRune):
			return []model.Finding{deny("en-dash", "en dash (U+2013) in the commit message; use a comma, a colon or a hyphen")}, nil
		}
	}
	return nil, nil
}
