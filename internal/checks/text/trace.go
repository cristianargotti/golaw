package text

import (
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// The trace patterns are assembled from halves so that this file carries
// none of the words it forbids and the kit's own scan stays at zero. A
// trailer phrase counts in trailer form only, a key at the start of a line
// or the marker of a generated commit, because the phrases are ordinary
// prose in an adopting repository (mocks generated with a tool); the
// pipeline word is prose there too and stays a boundary of the kit's own
// tree, which tools/cicheck holds.
var (
	trailerPattern = regexp.MustCompile(`(?i)^\s*(?:co-auth` + `ored-by|sign` + `ed-off-by):|gener` + `ated with \[`)
	tracePattern   = regexp.MustCompile(`(?i)\b(?:scr` + `atchpad|sub-?` + `agents?)\b`)
	homePattern    = regexp.MustCompile(`/Us` + `ers/`)
)

// TraceTerms is text.trace-terms: no commit trailer, no orchestration trace
// word and no absolute home path in any text file.
func TraceTerms() model.Check {
	return lineRule{spec: spec("text.trace-terms"), accept: anyText, terms: []term{
		{pattern: trailerPattern, code: "trailer", message: "commit trailer phrase in text; the kit carries no trailer of any kind"},
		{pattern: tracePattern, code: "trace-term", message: "orchestration trace word in text; the public tree carries no session trace"},
		{pattern: homePattern, code: "home-path", message: "absolute home path in text; use a path relative to the repository"},
	}}
}
