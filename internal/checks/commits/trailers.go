package commits

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/cristianargotti/golaw/internal/model"
)

// trailerPhrase reads the trailers the law names in trailer form: a
// co-author or sign-off key at the start of a line, or the marker of a
// generated commit before its link bracket at the start of a line, where a
// tool writes it, with nothing but white space or an emoji in front. The
// pattern is assembled from halves so this file carries none of them, and
// the words in prose (mocks a tool generated, a sentence or a list item
// that links the tool) are no trailer.
var trailerPhrase = regexp.MustCompile(`(?i)^\s*(?:co-auth` + `ored-by|sign` + `ed-off-by):|^[\s\p{So}]*gener` + `ated with \[`)

// NoTrailers is commit.no-trailers: no trailer of any kind. It reads the
// trailers of the law on every line and every trailer git itself reads in
// the last paragraph, whatever its key, and names each line once.
func NoTrailers() model.Check {
	return messageRule{spec: spec("commit.no-trailers"), judge: judgeTrailers}
}

func judgeTrailers(message, comment string) []model.Finding {
	ls := lines(message)
	keys := gitTrailers(ls, comment)
	var out []model.Finding
	for i, line := range ls {
		key, found := keys[i]
		if phrase := trailerPhrase.FindString(line); phrase != "" {
			key, found = strings.TrimFunc(phrase, notLetter), true
		}
		if found {
			text := fmt.Sprintf("line %d is a commit trailer (%s); the log carries no trailer of any kind: remove the line or reword it", i+1, key)
			out = append(out, deny("trailer", i+1, 1, text))
		}
	}
	return out
}

// notLetter trims a phrase to its words, so the reason names the marker
// alike with or without the emoji in front of it.
func notLetter(r rune) bool { return !unicode.IsLetter(r) }
