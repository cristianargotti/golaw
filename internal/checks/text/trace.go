package text

import (
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// The trace patterns are assembled from halves so that this file carries
// none of the words it forbids and the kit's own scan stays at zero.
var (
	trailerPattern = regexp.MustCompile(`(?i)co-auth` + `ored-by|gener` + `ated with|sign` + `ed-off-by`)
	tracePattern   = regexp.MustCompile(`(?i)\b(?:scr` + `atchpad|sub-?` + `agents?|work` + `flow)\b`)
	homePattern    = regexp.MustCompile(`/Us` + `ers/`)
)

// TraceTerms is text.trace-terms: no commit trailer phrase, no orchestration
// trace word and no absolute home path in any text file.
func TraceTerms() model.Check {
	return lineRule{spec: spec("text.trace-terms"), accept: anyText, terms: []term{
		{pattern: trailerPattern, code: "trailer", message: "commit trailer phrase in text; the kit carries no trailer of any kind"},
		{pattern: tracePattern, code: "trace-term", message: "orchestration trace word in text; the public tree carries no session trace"},
		{pattern: homePattern, code: "home-path", message: "absolute home path in text; use a path relative to the repository"},
	}}
}
