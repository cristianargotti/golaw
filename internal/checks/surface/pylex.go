package surface

import "bytes"

// pyToken is one lexeme of Python: its kind, its text, the 1-based lines
// of its first and last bytes and the 0-based byte column of its first.
type pyToken struct {
	kind    tokenKind
	text    string
	line    int
	endLine int
	col     int
}

// pyPunctuators are the multi-byte operators of Python, longest first.
func pyPunctuators() []string {
	return []string{
		"**=", "//=", ">>=", "<<=", ":=", "->", "==", "!=", "<=", ">=", "**", "//",
		"<<", ">>", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "@=",
	}
}

// The line breaks a joining backslash may stand before.
const (
	lf   = "\n"
	crlf = "\r\n"
)

// tripleQuote is the length of the quote of a triple quoted string.
const tripleQuote = 3

// pyLexer scans one Python file; lineStart is the offset of the first byte
// of the current line, from which a column is counted.
type pyLexer struct {
	src       []byte
	pos       int
	line      int
	lineStart int
	out       []pyToken
}

// lexPython answers the tokens of a Python file. It is total: an
// unterminated string runs to the end of its line, or of the file for a
// triple quoted one, so every input yields tokens and never an error.
func lexPython(src []byte) []pyToken {
	l := &pyLexer{src: src, line: 1}
	for l.pos < len(l.src) {
		l.next()
	}
	return l.out
}

// next consumes one token or one byte of space.
func (l *pyLexer) next() {
	c := l.src[l.pos]
	switch {
	case c == '\n':
		l.newline(l.pos + 1)
	case c == ' ' || c == '\t' || c == '\r' || c == '\f':
		l.pos++
	case c == '\\' && l.joinsLines():
	case c == '#':
		l.comment()
	case c == '\'' || c == '"' || (isLetter(c) && l.prefixed()):
		l.str()
	case isLetter(c) || c == '_' || c >= 0x80:
		l.word(tokIdent)
	case isDigit(c) || (c == '.' && l.pos+1 < len(l.src) && isDigit(l.src[l.pos+1])):
		l.word(tokNumber)
	default:
		l.punct()
	}
}

// newline moves past a line break whose next line starts at start.
func (l *pyLexer) newline(start int) {
	l.line++
	l.pos, l.lineStart = start, start
}

// joinsLines steps over a backslash that joins its line to the next, and
// reports whether it did.
func (l *pyLexer) joinsLines() bool {
	rest := l.src[l.pos+1:]
	switch {
	case bytes.HasPrefix(rest, []byte(lf)):
		l.newline(l.pos + 1 + len(lf))
	case bytes.HasPrefix(rest, []byte(crlf)):
		l.newline(l.pos + 1 + len(crlf))
	default:
		return false
	}
	return true
}

// emit appends a token that starts at start on line, ending at the
// position on the current line.
func (l *pyLexer) emit(kind tokenKind, start, line, col int) {
	l.out = append(l.out, pyToken{kind: kind, text: string(l.src[start:l.pos]), line: line, endLine: l.line, col: col})
}

// comment scans a comment to the end of its line.
func (l *pyLexer) comment() {
	start, col := l.pos, l.pos-l.lineStart
	if end := bytes.IndexByte(l.src[l.pos:], '\n'); end >= 0 {
		l.pos += end
	} else {
		l.pos = len(l.src)
	}
	l.emit(tokComment, start, l.line, col)
}

// word scans an identifier or a number, a number also taking its dots.
func (l *pyLexer) word(kind tokenKind) {
	start, col := l.pos, l.pos-l.lineStart
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if !isLetter(c) && !isDigit(c) && c != '_' && c < 0x80 && (kind != tokNumber || c != '.') {
			break
		}
		l.pos++
	}
	l.emit(kind, start, l.line, col)
}

// punct scans the longest operator at the position, else one byte.
func (l *pyLexer) punct() {
	start, col := l.pos, l.pos-l.lineStart
	l.pos++
	for _, p := range pyPunctuators() {
		if bytes.HasPrefix(l.src[start:], []byte(p)) {
			l.pos = start + len(p)
			break
		}
	}
	l.emit(tokPunct, start, l.line, col)
}

// pySignificant drops the comments, which hold no code.
func pySignificant(tokens []pyToken) []pyToken {
	out := make([]pyToken, 0, len(tokens))
	for _, t := range tokens {
		if t.kind != tokComment {
			out = append(out, t)
		}
	}
	return out
}
