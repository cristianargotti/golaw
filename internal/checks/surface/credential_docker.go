package surface

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

const (
	keywordEnv = "ENV"
	keywordArg = "ARG"
	// dockerTail is the fix of the S6472 message (M6.6.5).
	dockerTail = " (Sonar S6472): pass it as a build secret or at run time from the environment."
)

// judgeDockerCredential reads the ENV pairs and the ARG defaults of a
// Dockerfile: one finding per instruction that gives a credential name a
// value that is not benign.
func judgeDockerCredential(f source) []model.Finding {
	var out []model.Finding
	for _, ins := range readInstructions(f.src) {
		if ins.keyword != keywordEnv && ins.keyword != keywordArg {
			continue
		}
		for _, pair := range dockerPairs(ins) {
			if credentialName(pair[0]) && !benign(pair[0], pair[1]) {
				message := "sets a literal default for " + pair[0] + dockerTail
				out = append(out, f.finding(literalCode, ins.line, message, sonarKey(detect.Dockerfile, "S6472")))
				break
			}
		}
	}
	return out
}

// dockerPairs answers the KEY and value pairs an instruction sets: every
// KEY=value word, and for ENV the legacy KEY value form when the first word
// holds no equals sign. An ARG without a default sets nothing.
func dockerPairs(ins instruction) [][2]string {
	words := dockerWords(ins.args)
	if len(words) == 0 {
		return nil
	}
	if ins.keyword == keywordEnv && !strings.Contains(words[0], "=") {
		key, rest := splitWord(ins.args)
		return [][2]string{{key, unquote(rest)}}
	}
	var out [][2]string
	for _, w := range words {
		if key, val, ok := strings.Cut(w, "="); ok {
			out = append(out, [2]string{key, unquote(val)})
		}
	}
	return out
}

// dockerWords splits the arguments on blanks outside quotes.
func dockerWords(args string) []string {
	var words []string
	var quote byte
	start := -1
	for i := range len(args) {
		c := args[i]
		switch {
		case quote != 0:
			quote = closeQuote(quote, c)
		case c == ' ' || c == '\t':
			if start >= 0 {
				words = append(words, args[start:i])
			}
			start = -1
		default:
			if start < 0 {
				start = i
			}
			quote = openQuote(c)
		}
	}
	if start >= 0 {
		words = append(words, args[start:])
	}
	return words
}

// openQuote answers the quote a byte opens, 0 for any other byte.
func openQuote(c byte) byte {
	if c == '"' || c == '\'' {
		return c
	}
	return 0
}

// closeQuote answers the quote still open after a byte inside it.
func closeQuote(quote, c byte) byte {
	if c == quote {
		return 0
	}
	return quote
}

// unquote drops the double or single quotes around a value.
func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}
