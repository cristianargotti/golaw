package surface

import (
	"bytes"
	"slices"
	"strings"
)

// stringPrefixes are the prefixes of a Python string, lower case.
func stringPrefixes() []string {
	return []string{"r", "u", "b", "f", "rb", "br", "fr", "rf"}
}

// prefixed reports whether the identifier bytes at the position are a
// string prefix directly before a quote.
func (l *pyLexer) prefixed() bool {
	_, ok := prefixLength(l.src[l.pos:])
	return ok
}

// prefixLength answers the length of the string prefix that src starts
// with, and whether one starts it at all.
func prefixLength(src []byte) (int, bool) {
	n := 0
	for n < len(src) && n < tripleQuote && isLetter(src[n]) {
		n++
	}
	if n == 0 || n == tripleQuote || n >= len(src) || (src[n] != '\'' && src[n] != '"') {
		return 0, false
	}
	return n, slices.Contains(stringPrefixes(), strings.ToLower(string(src[:n])))
}

// str scans a string from its prefix: a triple quoted one to the same
// triple or the end of the file, a single quoted one to the same quote,
// before its line break or at the end. A backslash steps over the next
// byte in every string, so an escaped quote never ends one, raw or not.
func (l *pyLexer) str() {
	start, line, col := l.pos, l.line, l.pos-l.lineStart
	n, _ := prefixLength(l.src[l.pos:])
	l.pos += n
	quote := l.src[l.pos]
	triple := bytes.HasPrefix(l.src[l.pos:], bytes.Repeat([]byte{quote}, tripleQuote))
	if triple {
		l.pos += tripleQuote
	} else {
		l.pos++
	}
	l.body(quote, triple)
	l.emit(tokString, start, line, col)
}

// body scans the body of a string up to and past its closing quote.
func (l *pyLexer) body(quote byte, triple bool) {
	closing := []byte{quote}
	if triple {
		closing = bytes.Repeat(closing, tripleQuote)
	}
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case bytes.HasPrefix(l.src[l.pos:], closing):
			l.pos += len(closing)
			return
		case c == '\n' && !triple:
			return
		case c == '\n':
			l.newline(l.pos + 1)
		case c == '\\' && l.pos+1 < len(l.src):
			if l.src[l.pos+1] == '\n' {
				l.newline(l.pos + 2)
			} else {
				l.pos += 2
			}
		default:
			l.pos++
		}
	}
}

// pyStringValue answers the value of a string token without its prefix and
// quotes, and false for an f-string that holds a substitution, whose value
// is built at run time. A string that is not raw resolves a backslash
// before a quote or a backslash.
func pyStringValue(t pyToken) (string, bool) {
	n, _ := prefixLength([]byte(t.text))
	prefix, text := strings.ToLower(t.text[:n]), t.text[n:]
	if text == "" {
		return "", true
	}
	quote := string(text[0])
	if strings.HasPrefix(text, strings.Repeat(quote, tripleQuote)) {
		quote = strings.Repeat(quote, tripleQuote)
	}
	text = strings.TrimSuffix(strings.TrimPrefix(text, quote), quote)
	if strings.Contains(prefix, "f") && strings.Contains(strings.ReplaceAll(text, "{{", ""), "{") {
		return "", false
	}
	if strings.Contains(prefix, "r") {
		return text, true
	}
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) && strings.IndexByte(`\'"`, text[i+1]) >= 0 {
			i++
		}
		out = append(out, text[i])
	}
	return string(out), true
}
