package shell

import "strings"

// scan reads a command line of a POSIX shell or of PowerShell into its
// segments, without reading the scripts they run, and keeps the text the
// shell expands while it runs them (substitutions).
func scan(command string, d dialect) *scanner {
	s := &scanner{escapeRune: rune(d)}
	for _, r := range command {
		s.step(r)
	}
	s.endInput()
	return s
}

// scanner reads the command one rune at a time, because a quote, a
// backslash or a separator changes what the next rune means and a split on
// blanks would cut a quoted message at its spaces.
type scanner struct {
	out        []segment
	cur        segment
	word       []rune
	open       bool      // a word is being built; an empty quoted string counts
	literal    bool      // the word holds a quoted or escaped rune, so it names no stream
	quote      rune      // the active quote, 0 outside quotes
	escape     bool      // the next rune is taken literally
	escapeRune rune      // the escape of the dialect, 0 for none
	amp        bool      // an ampersand that starts the operator &> when one follows
	op         []rune    // the redirection operator being read
	pending    string    // a whole operator whose file is the next word
	redirects  []segment // the redirections of the command, each its operator and file
	docs       documents // the here-documents of the line and their bodies
	line       []rune    // the runes of the line outside here-document bodies
	bodies     []string  // the bodies of the documents whose end word is unquoted
	parens     int       // the parentheses open outside quotes
	cases      []int     // for each case open, the parentheses open where it began
	closed     bool      // a parenthesis closed none the scanner read open
	level      int       // the substitutions inside double quotes this scanner reads within
	inner      *scanner  // the reader of the substitution inside double quotes being read
	dollar     bool      // the last rune was a dollar inside double quotes
	ticks      bool      // a backquote inside double quotes opened a substitution
	comment    comment   // the comment being read, none outside one
	hash       bool      // the last rune of a block comment was a hash
}

func (s *scanner) step(r rune) {
	if s.diverted(r) || s.operatorRune(r) {
		return
	}
	switch {
	case s.escape:
		s.escaped(r)
	case s.quote != 0:
		s.quoted(r)
	case s.escapeRune != 0 && r == s.escapeRune:
		s.escape, s.open, s.literal = true, true, true
	case r == '\'' || r == '"':
		s.quote, s.open, s.literal = r, true, true
	case r == '<' || r == '>' || len(s.op) > 0:
		s.redirect(r)
	case r == '&':
		s.amp = true
	case r == ' ' || r == '\t':
		s.endWord()
	case r == '\n':
		s.endSegment()
		s.newLine()
	case r == '(' || r == ')':
		s.paren(r)
	case r == '#' && !s.open:
		s.closeWord()
		s.comment = lineComment
	case strings.ContainsRune(separators, r):
		s.endSegment()
	default:
		s.open = true
		s.word = append(s.word, r)
	}
}

// escaped takes the rune after an escape literally; an escape before a
// newline joins two lines and adds nothing. Inside double quotes a POSIX
// backslash escapes only $, the backquote, the double quote, itself and the
// newline, and stays before any other rune, as the shell leaves it there.
func (s *scanner) escaped(r rune) {
	s.escape = false
	if s.quote == '"' && s.escapeRune == '\\' && !strings.ContainsRune("$`\"\\\n", r) {
		s.word = append(s.word, '\\')
	}
	if r != '\n' {
		s.word = append(s.word, r)
	}
}

// quoted consumes a rune inside quotes: the closing quote ends them, the
// escape of the dialect inside double quotes escapes the next rune, and
// there a $( or, in a POSIX shell, a backquote opens a substitution, read
// apart (nested, tick).
func (s *scanner) quoted(r rune) {
	dollar := s.dollar
	s.dollar = false
	switch {
	case r == s.quote:
		s.quote = 0
	case s.quote != '"':
		s.word = append(s.word, r)
	case s.escapeRune != 0 && r == s.escapeRune:
		s.escape = true
	case r == '`':
		s.ticks = true
		s.word = append(s.word, r)
	case r == '(' && dollar && s.level < maxDepth:
		s.inner = &scanner{escapeRune: s.escapeRune, level: s.level + 1}
		s.word = append(s.word, r)
	default:
		s.dollar = r == '$'
		s.word = append(s.word, r)
	}
}

// endWord ends the word being built: the file of the operator waiting for
// one, or the end word of a here-document in a POSIX shell, or a word of
// the command.
func (s *scanner) endWord() {
	switch {
	case s.open && s.pending != "":
		s.redirects = append(s.redirects, segment{s.pending, string(s.word)})
		if doc, strip := heredocOperator(s.pending); doc && s.escapeRune == rune(posix) {
			s.docs.open = append(s.docs.open, heredoc{end: string(s.word), strip: strip, quoted: s.literal})
		}
		s.pending = ""
	case s.open:
		if len(s.cur) == 0 && !s.literal {
			s.keyword(string(s.word))
		}
		s.cur = append(s.cur, string(s.word))
	}
	s.word, s.open, s.literal = s.word[:0], false, false
}

// endSegment ends the command being read, then its redirections, each a
// segment of its own.
func (s *scanner) endSegment() {
	if len(s.op) > 0 {
		s.pending, s.op = string(s.op), s.op[:0]
	}
	s.closeWord()
	if len(s.cur) > 0 {
		s.out = append(s.out, s.cur)
	}
	s.out = append(s.out, s.redirects...)
	s.cur, s.redirects, s.amp = nil, nil, false
}

// keyword reads the word a command starts with: a case opens patterns, each
// ended by a closing parenthesis at the depth where the case began, up to
// its esac.
func (s *scanner) keyword(word string) {
	switch {
	case word == "case":
		s.cases = append(s.cases, s.parens)
	case word == "esac" && len(s.cases) > 0:
		s.cases = s.cases[:len(s.cases)-1]
	}
}

// paren reads a parenthesis outside quotes, which ends a segment: an
// opening one nests, and a closing one ends a case pattern at the depth
// where its case began, closes the one it matches or, with none open,
// marks the end of the substitution the scanner reads (enclosed).
func (s *scanner) paren(r rune) {
	s.endSegment()
	switch {
	case r == '(':
		s.parens++
	case len(s.cases) > 0 && s.cases[len(s.cases)-1] == s.parens:
	case s.parens == 0:
		s.closed = true
	default:
		s.parens--
	}
}
