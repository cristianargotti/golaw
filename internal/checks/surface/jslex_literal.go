package surface

import "bytes"

// lineComment scans a comment to the end of its line, the line break left
// for the space rule to count.
func (l *lexer) lineComment() {
	start := l.pos
	end := bytes.IndexByte(l.src[l.pos:], '\n')
	if end < 0 {
		l.pos = len(l.src)
	} else {
		l.pos += end
	}
	l.emit(tokComment, string(l.src[start:l.pos]), l.line)
}

// blockComment scans a comment to its closing mark or the end of the unit.
func (l *lexer) blockComment() {
	start, line := l.pos, l.line
	end := bytes.Index(l.src[l.pos+2:], []byte("*/"))
	if end < 0 {
		l.pos = len(l.src)
	} else {
		l.pos += 2 + end + 2
	}
	l.line += bytes.Count(l.src[start:l.pos], []byte("\n"))
	l.emit(tokComment, string(l.src[start:l.pos]), line)
}

// quoted scans a string to the same quote not escaped by a backslash; an
// unterminated string ends before its line break or at the end.
func (l *lexer) quoted(quote byte) {
	start, line := l.pos, l.line
	l.pos++
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '\n' {
			break
		}
		l.pos++
		if c == quote {
			break
		}
		if c == '\\' && l.pos < len(l.src) {
			l.skipEscaped()
		}
	}
	l.emit(tokString, string(l.src[start:l.pos]), line)
}

// skipEscaped steps over the byte a backslash escapes, counting a line
// continuation.
func (l *lexer) skipEscaped() {
	if l.src[l.pos] == '\n' {
		l.line++
	}
	l.pos++
}

// regex scans a regular expression to the slash that closes it outside a
// class, then its flags; an unterminated one ends before its line break.
func (l *lexer) regex() {
	start := l.pos
	l.pos++
	class := false
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		c := l.src[l.pos]
		l.pos++
		switch {
		case c == '\\' && l.pos < len(l.src) && l.src[l.pos] != '\n':
			l.pos++
		case c == '[':
			class = true
		case c == ']':
			class = false
		case c == '/' && !class:
			for l.pos < len(l.src) && isLetter(l.src[l.pos]) {
				l.pos++
			}
			l.emit(tokRegex, string(l.src[start:l.pos]), l.line)
			return
		}
	}
	l.emit(tokRegex, string(l.src[start:l.pos]), l.line)
}

// template scans a template literal whose text is the whole literal; the
// tokens of each substitution follow the template token, so the token is
// placed first and its text filled once the closing backtick is read.
func (l *lexer) template() {
	start, at := l.pos, len(l.out)
	l.out = append(l.out, token{kind: tokTemplate, line: l.line})
	l.pos++
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		l.pos++
		switch {
		case c == '`':
			l.closeTemplate(at, start)
			return
		case c == '\n':
			l.line++
		case c == '\\' && l.pos < len(l.src):
			l.skipEscaped()
		case c == '$' && l.pos < len(l.src) && l.src[l.pos] == '{':
			l.pos++
			l.out[at].sub = true
			l.expr = true
			l.until()
			if l.pos < len(l.src) {
				l.pos++
			}
		}
	}
	l.closeTemplate(at, start)
}

// closeTemplate fills the text of the template token placed at index at;
// a template is an operand, so no expression starts after it.
func (l *lexer) closeTemplate(at, start int) {
	l.out[at].text = string(l.src[start:l.pos])
	l.expr = false
}

// until lexes an expression up to the closing brace that matches none of
// its own, leaving the position on that brace, or to the end of the unit.
func (l *lexer) until() {
	depth := 0
	for l.pos < len(l.src) {
		switch l.src[l.pos] {
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return
			}
			depth--
		}
		l.next()
	}
}
