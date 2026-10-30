package surface

// element scans a JSX element whose less-than sign is at the position: the
// tag name, the attributes, and the children unless it closes itself. A
// name followed by a comma or extends is the type parameter list of a
// generic arrow in a TSX file, so the sign is an operator and the scan
// goes on from the name.
func (l *lexer) element() {
	line := l.line
	l.pos++
	start := l.pos
	for l.pos < len(l.src) && tagPart(l.src[l.pos]) {
		l.pos++
	}
	name := string(l.src[start:l.pos])
	if l.generic() {
		l.pos = start
		l.emit(tokPunct, "<", line)
		return
	}
	if name != "" {
		l.emit(tokTag, name, line)
	}
	if l.attributes() {
		l.children()
	}
	l.expr = false
}

// generic reports whether the element head just read is a type parameter
// list: the name is followed by a comma or the word extends.
func (l *lexer) generic() bool {
	i := l.pos
	for i < len(l.src) && (l.src[i] == ' ' || l.src[i] == '\t') {
		i++
	}
	if i < len(l.src) && l.src[i] == ',' {
		return true
	}
	rest := l.src[i:]
	return len(rest) > len(keywordExtends) && string(rest[:len(keywordExtends)]) == keywordExtends &&
		!identPart(rest[len(keywordExtends)])
}

// attributes scans the attributes of an element head and reports whether
// the element opens children; a self-closing element or the end of the
// unit opens none.
func (l *lexer) attributes() bool {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			l.line++
			l.pos++
		case l.at("/>"):
			l.pos += 2
			return false
		case c == '>':
			l.pos++
			return true
		case c == '{':
			l.container()
		case c == '"' || c == '\'':
			l.attributeValue(c)
		case c == '=':
			l.pos++
			l.emit(tokPunct, "=", l.line)
		case identStart(c):
			l.attributeName()
		default:
			l.pos++
		}
	}
	return false
}

// attributeName scans a name that may hold dashes and colons.
func (l *lexer) attributeName() {
	start := l.pos
	for l.pos < len(l.src) && tagPart(l.src[l.pos]) {
		l.pos++
	}
	l.emit(tokIdent, string(l.src[start:l.pos]), l.line)
}

// attributeValue scans a quoted value, which knows no escapes and may span
// lines.
func (l *lexer) attributeValue(quote byte) {
	start, line := l.pos, l.line
	l.pos++
	for l.pos < len(l.src) && l.src[l.pos] != quote {
		if l.src[l.pos] == '\n' {
			l.line++
		}
		l.pos++
	}
	l.pos = min(l.pos+1, len(l.src))
	l.emit(tokString, string(l.src[start:l.pos]), line)
}

// children scans the children of an element up to its closing tag, which
// closes one level and emits nothing.
func (l *lexer) children() {
	for l.pos < len(l.src) {
		switch {
		case l.at("</"):
			for l.pos < len(l.src) && l.src[l.pos] != '>' {
				l.pos++
			}
			l.pos = min(l.pos+1, len(l.src))
			return
		case l.src[l.pos] == '<' && (isLetter(l.following()) || l.following() == '>'):
			l.element()
		case l.src[l.pos] == '{':
			l.container()
		default:
			l.text()
		}
	}
}

// text scans the text of children up to the next element or container; a
// run of spaces alone is no token.
func (l *lexer) text() {
	start, line := l.pos, l.line
	l.pos++
	for l.pos < len(l.src) && l.src[l.pos] != '<' && l.src[l.pos] != '{' {
		l.pos++
	}
	body := l.src[start:l.pos]
	for _, c := range body {
		if c == '\n' {
			l.line++
		}
	}
	for _, c := range body {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			l.emit(tokText, string(body), line)
			return
		}
	}
}

// container scans an expression container, its braces emitted.
func (l *lexer) container() {
	l.pos++
	l.emit(tokPunct, "{", l.line)
	l.until()
	if l.pos < len(l.src) {
		l.pos++
		l.emit(tokPunct, "}", l.line)
	}
}

// keywordExtends opens the constraint of a type parameter.
const keywordExtends = "extends"

// tagPart reports a byte of a tag or attribute name.
func tagPart(c byte) bool { return identPart(c) || c == '-' || c == ':' || c == '.' }
