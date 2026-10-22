package codex

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// token is one element of the line that opens a here-document form: an
// operator the recognizer reads, or a word with its literal value; quoted
// tells a word some part of which the shell does not expand.
type token struct {
	op     string
	word   string
	quoted bool
}

// The operators of a recognized form; any other shell operator, the here
// string and the tab-stripping here-document included, ends the reading.
const (
	andOp     = "&&"
	writeOp   = ">"
	appendOp  = ">>"
	heredocOp = "<<"
)

// lexLine splits the opening line of a form into its tokens. Only literal
// words are read: a word with an expansion, a substitution, a glob, an
// escape or a comment answers false, because its value is the shell's to
// decide and the recognizer never expands one.
func lexLine(line string) ([]token, bool) {
	var tokens []token
	for rest := strings.TrimLeft(line, " \t"); rest != ""; rest = strings.TrimLeft(rest, " \t") {
		next, n, ok := lexToken(rest)
		if !ok {
			return nil, false
		}
		tokens, rest = append(tokens, next), rest[n:]
	}
	return tokens, true
}

// lexToken reads the operator or the word at the start of a text and the
// number of bytes it spans.
func lexToken(text string) (token, int, bool) {
	for _, op := range []string{andOp, appendOp, heredocOp, writeOp} {
		if strings.HasPrefix(text, op) {
			if refusedAfter(op, text[len(op):]) {
				return token{}, 0, false
			}
			return token{op: op}, len(op), true
		}
	}
	if strings.ContainsRune("&|;<>()#", rune(text[0])) {
		return token{}, 0, false
	}
	return lexWord(text)
}

// refusedAfter tells the operators the recognizer leaves to the shell from
// the ones it reads: <<- and <<< after <<, and >| and >& after > or >>.
func refusedAfter(op, rest string) bool {
	if rest == "" {
		return false
	}
	switch op {
	case heredocOp:
		return rest[0] == '-' || rest[0] == '<'
	case writeOp, appendOp:
		return rest[0] == '|' || rest[0] == '&' || rest[0] == '>'
	default:
		return false
	}
}

// lexWord reads one word made of unquoted, single-quoted and double-quoted
// segments, up to white space or an operator.
func lexWord(text string) (token, int, bool) {
	var word token
	i := 0
	for i < len(text) && !strings.ContainsRune(" \t&|;<>()", rune(text[i])) {
		value, quoted, n := lexSegment(text[i:])
		if n == 0 {
			return token{}, 0, false
		}
		word.word, word.quoted, i = word.word+value, word.quoted || quoted, i+n
	}
	return word, i, true
}

// lexSegment reads one quoted string, or one unquoted literal character,
// and answers its value, whether it was quoted and the bytes it spans,
// none when the shell would expand it.
func lexSegment(text string) (string, bool, int) {
	switch text[0] {
	case '\'':
		if end := strings.IndexByte(text[1:], '\''); end >= 0 {
			return text[1 : 1+end], true, end + 2
		}
		return "", false, 0
	case '"':
		end := strings.IndexByte(text[1:], '"')
		if end < 0 || strings.ContainsAny(text[1:1+end], "$`\\") {
			return "", false, 0
		}
		return text[1 : 1+end], true, end + 2
	}
	r, n := utf8.DecodeRuneInString(text)
	if !literalRune(r) {
		return "", false, 0
	}
	return text[:n], false, n
}

// literalRune tells a character the shell keeps as it is outside quotes:
// a letter, a digit or punctuation that starts no expansion, glob, brace,
// history or escape. A ~ is literal but at the start of a word, where the
// caller reads it: a cat target there is left to the shell, while Codex
// joins a cd word that starts with it literally.
func literalRune(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	return strings.ContainsRune("-_./+:,@%=^~", r)
}
