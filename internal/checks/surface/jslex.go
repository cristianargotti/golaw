package surface

import (
	"bytes"
	"slices"
)

// tokenKind says what a token of a lexer is.
type tokenKind int

// The token kinds the lexers of the family emit.
const (
	tokIdent tokenKind = iota
	tokNumber
	tokString
	tokTemplate
	tokRegex
	tokPunct
	tokComment
	tokTag
	tokText
)

// String names the kind for a test or a report line.
func (k tokenKind) String() string {
	names := []string{"ident", "number", "string", "template", "regex", "punct", "comment", "tag", "text"}
	if int(k) < 0 || int(k) >= len(names) {
		return "unknown"
	}
	return names[k]
}

// token is one lexeme: its kind, its text, the 1-based line of its first
// byte, and whether a template holds a substitution.
type token struct {
	kind tokenKind
	text string
	line int
	sub  bool
}

// keywordNew is the keyword that constructs, which the checks read before a
// callee.
const keywordNew = "new"

// punctuators are the multi-byte operators, longest first, so an operator
// is never split into two.
func punctuators() []string {
	return []string{
		">>>=", "===", "!==", "**=", "<<=", ">>=", ">>>", "...", "&&=", "||=", "??=",
		"=>", "==", "!=", "<=", ">=", "&&", "||", "??", "?.", "++", "--", "+=", "-=",
		"*=", "/=", "%=", "&=", "|=", "^=", "**", "<<", ">>",
	}
}

// exprKeywords are the keywords after which an expression starts, so a
// slash there opens a regular expression and a less-than sign an element.
func exprKeywords() []string {
	return []string{
		"return", "typeof", "instanceof", "in", "of", keywordNew, "delete", "void",
		"throw", "case", "do", "else", "yield", "await",
	}
}

// lexer scans one unit of JavaScript, TypeScript, JSX or TSX. expr is true
// where an expression may start, the only fact the grammar needs to tell a
// regular expression from a division and an element from a comparison.
type lexer struct {
	src  []byte
	pos  int
	line int
	jsx  bool
	expr bool
	out  []token
}

// lexJS answers the tokens of a unit whose first line is firstLine. It is
// total: an unterminated literal or comment runs to the end of its line or
// unit, so every input yields tokens and never an error.
func lexJS(src []byte, firstLine int, jsx bool) []token {
	l := &lexer{src: src, line: firstLine, jsx: jsx, expr: true}
	if bytes.HasPrefix(src, []byte("#!")) {
		l.lineComment()
	}
	for l.pos < len(l.src) {
		l.next()
	}
	return l.out
}

// next consumes one token or one byte of space.
func (l *lexer) next() {
	c := l.src[l.pos]
	switch {
	case c == '\n':
		l.line++
		l.pos++
	case c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v':
		l.pos++
	case l.at("//") || l.at("<!--") || (l.at("-->") && l.lineStart()):
		l.lineComment()
	case l.at("/*"):
		l.blockComment()
	case identStart(c):
		l.word(tokIdent)
	case isDigit(c) || (c == '.' && isDigit(l.following())):
		l.word(tokNumber)
	case c == '\'' || c == '"':
		l.quoted(c)
	case c == '`':
		l.template()
	case c == '/' && l.expr:
		l.regex()
	case c == '<' && l.jsx && l.expr && (isLetter(l.following()) || l.following() == '>'):
		l.element()
	default:
		l.punct()
	}
}

// emit appends a token that starts at line and keeps expr current.
func (l *lexer) emit(kind tokenKind, text string, line int) {
	l.out = append(l.out, token{kind: kind, text: text, line: line})
	if kind == tokComment || kind == tokText {
		return
	}
	l.expr = (kind == tokPunct && text != ")" && text != "]" && text != "}") ||
		(kind == tokIdent && slices.Contains(exprKeywords(), text))
}

// at reports whether the source continues with s at the position.
func (l *lexer) at(s string) bool { return bytes.HasPrefix(l.src[l.pos:], []byte(s)) }

// following answers the byte after the position, 0 past the end.
func (l *lexer) following() byte {
	if l.pos+1 < len(l.src) {
		return l.src[l.pos+1]
	}
	return 0
}

// lineStart reports whether only spaces precede the position on its line,
// where an HTML-like closing comment is one.
func (l *lexer) lineStart() bool {
	start := bytes.LastIndexByte(l.src[:l.pos], '\n') + 1
	return len(bytes.TrimSpace(l.src[start:l.pos])) == 0
}

// word scans an identifier or a number: the same run of word bytes, a
// number also taking the dots of a fraction.
func (l *lexer) word(kind tokenKind) {
	start := l.pos
	for l.pos < len(l.src) && (identPart(l.src[l.pos]) || (kind == tokNumber && l.src[l.pos] == '.')) {
		l.pos++
	}
	l.emit(kind, string(l.src[start:l.pos]), l.line)
}

// punct scans the longest operator at the position, else one byte.
func (l *lexer) punct() {
	for _, p := range punctuators() {
		if l.at(p) {
			l.pos += len(p)
			l.emit(tokPunct, p, l.line)
			return
		}
	}
	l.pos++
	l.emit(tokPunct, string(l.src[l.pos-1]), l.line)
}

func identStart(c byte) bool {
	return isLetter(c) || c == '_' || c == '$' || c == '#' || c >= 0x80
}

func identPart(c byte) bool { return identStart(c) || isDigit(c) }

func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
