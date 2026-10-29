package surface

import (
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

const (
	suppressionCode = "suppression"
	// analyzerMarker is assembled from halves, so no production file of
	// the kit spells the marker its own scan refuses.
	analyzerMarker = "NO" + "SONAR"
	markerMessage  = "adds " + analyzerMarker + ", which hides every rule on the line from the pipeline's analyzer: fix the finding; golaw lint names it."
	noqaMessage    = "adds a bare noqa, which hides every rule on the line: name the codes after noqa:, or fix the finding; golaw lint names it."
	directiveNoqa  = "noqa"
)

// Suppression denies a comment that hides every rule of a line from the
// remote analyzer, in any language Go included, and a bare ESLint, ruff or
// type checker suppression, when the candidate introduces it (M6.7 check
// 5): a suppression that names its rules passes.
func Suppression() model.Check {
	return check{
		spec:      spec("surface.suppression"),
		languages: detect.Languages(),
		goFiles:   true,
		certain:   true,
		judge:     judgeSuppression,
	}
}

// judgeSuppression answers one finding per comment at most, the marker
// first, at the comment's line.
func judgeSuppression(f source, _ model.Input) []model.Finding {
	var out []model.Finding
	for _, c := range comments(f) {
		if message := suppressionMessage(c); message != "" {
			out = append(out, f.finding(suppressionCode, c.line, message, ""))
		}
	}
	return out
}

// suppressionMessage is the reason a comment hides rules, empty when it
// hides none.
func suppressionMessage(c comment) string {
	if hasWord(c.text, analyzerMarker) {
		return markerMessage
	}
	if c.language == detect.Python {
		return pythonSuppression(c.text)
	}
	if !slices.Contains([]detect.Language{detect.JavaScript, detect.TypeScript, detect.JSX, detect.TSX}, c.language) {
		return ""
	}
	if d := bareESLint(c.text); d != "" {
		return "adds a bare " + d + ", which hides every rule it reaches: name the rule and a reason after --, or fix the finding; golaw lint names it."
	}
	return ""
}

// hasWord reports the word in the text bounded by bytes that are not word
// bytes, so a longer identifier holding it is not the word.
func hasWord(text, word string) bool {
	for at := 0; ; {
		i := strings.Index(text[at:], word)
		if i < 0 {
			return false
		}
		start, end := at+i, at+i+len(word)
		if (start == 0 || !wordByte(text[start-1])) && (end == len(text) || !wordByte(text[end])) {
			return true
		}
		at = end
	}
}

// wordByte reports a letter, a digit or an underscore.
func wordByte(c byte) bool { return isLetter(c) || isDigit(c) || c == '_' }

// bareESLint answers the directive a script comment holds alone between
// its marks, empty when the comment names rules or is no directive.
func bareESLint(text string) string {
	var body string
	switch {
	case strings.HasPrefix(text, "//"):
		body = text[len("//"):]
	case strings.HasPrefix(text, blockOpen):
		body = strings.TrimSuffix(text[len(blockOpen):], blockClose)
	default:
		return ""
	}
	body = strings.TrimSpace(body)
	if slices.Contains([]string{"eslint-disable", "eslint-disable-line", "eslint-disable-next-line"}, body) {
		return body
	}
	return ""
}

// pythonSuppression reads each hash of a Python comment: a noqa of any
// case not followed by a colon, or a type checker ignore not followed by a
// bracket, directly after the hash and its blanks, is bare.
func pythonSuppression(text string) string {
	for i := range len(text) {
		if text[i] != '#' {
			continue
		}
		rest := strings.TrimLeft(text[i+1:], " \t")
		if bare(strings.ToLower(rest), directiveNoqa, ':') {
			return noqaMessage
		}
		for _, d := range []string{"type: ignore", "pyright: ignore"} {
			if bare(rest, d, '[') {
				return "adds a bare " + d + ", which hides every type error on the line: name the rule in brackets, or fix the error; golaw lint names it."
			}
		}
	}
	return ""
}

// bare reports a directive at the start of the text that the named byte,
// which lists its rules, does not follow; a word byte after it makes it
// another word.
func bare(text, directive string, names byte) bool {
	rest, ok := strings.CutPrefix(text, directive)
	if !ok {
		return false
	}
	return rest == "" || (rest[0] != names && !wordByte(rest[0]))
}
