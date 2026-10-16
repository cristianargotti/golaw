package shell

import (
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

// The trailer phrase is assembled from halves so this file carries none; a
// trailer flag is --signoff, --trailer or a short cluster carrying s.
var (
	trailerPhrase = regexp.MustCompile(`(?i)co-auth` + `ored-by|gener` + `ated with|sign` + `ed-off-by`)
	trailerFlag   = regexp.MustCompile(`^(?:--signoff|--trailer(?:=.*)?|-[a-zA-Z]*s[a-zA-Z]*)$`)
)

// Trailers is shell.trailers: a commit made from the command line carries no
// trailer, neither as a phrase in its message nor as a flag.
func Trailers() model.Check {
	return commandRule{spec: spec("shell.trailers"), judge: judgeTrailers}
}

func judgeTrailers(_ model.Input, segs []segment) ([]model.Finding, error) {
	for _, call := range gitCalls(segs) {
		if isCommit(call) && (trailerPhrase.MatchString(messageOf(call)) || matchesArg(call, trailerFlag)) {
			return []model.Finding{deny("trailer", "the commit carries a trailer; the log carries no trailer of any kind")}, nil
		}
	}
	return nil, nil
}
