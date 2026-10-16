package shell

// substitutions answers the scripts of the command substitutions the shell
// runs while it expands the line the scanner read: those inside a word of
// the line outside here-document bodies, and every one of the bodies whose
// end word is unquoted.
func (s *scanner) substitutions() []string {
	d := dialect(s.escapeRune)
	out := expansions(s.line, d, false)
	for _, body := range s.bodies {
		out = append(out, expansions([]rune(body), d, true)...)
	}
	return out
}

// expansions answers the scripts of the substitutions a text holds: on a
// command line, a $( ) inside double quotes, which a POSIX shell and
// PowerShell both run, and in a POSIX shell the backquotes, outside quotes
// or inside double ones, since a substitution outside quotes is split by
// its parentheses already; in the body of a here-document, where quotes
// are text, every $( ) and backquote. The escape of the dialect makes the
// next rune text, and single quotes hold no substitution.
func expansions(runes []rune, d dialect, body bool) []string {
	var out []string
	quote := rune(0)
	if body {
		quote = '"'
	}
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; {
		case quote == '\'':
			if r == '\'' {
				quote = 0
			}
		case r == rune(d):
			i++
		case r == '`' && d == posix:
			end := ticked(runes, i+1)
			out = append(out, string(runes[i+1:end]))
			i = end
		case r == '$' && quote == '"' && i+1 < len(runes) && runes[i+1] == '(':
			end := enclosed(runes, i+2, d)
			out = append(out, string(runes[i+2:end]))
			i = end
		case body:
		case (r == '\'' || r == '"') && quote == 0:
			quote = r
		case r == '"':
			quote = 0
		}
	}
	return out
}

// ticked answers the index of the backquote that closes one opened before
// from, past the runes a backslash escapes, or the end of the text.
func ticked(runes []rune, from int) int {
	for i := from; i < len(runes); i++ {
		switch runes[i] {
		case '\\':
			i++
		case '`':
			return i
		}
	}
	return len(runes)
}

// enclosed answers the index of the parenthesis that closes a command
// substitution whose script starts at from, found the way the shell finds
// it: a scanner reads the script, so a parenthesis inside quotes, after an
// escape, in the body of a here-document or at the end of a case pattern
// closes nothing; the end of the text when none closes it.
func enclosed(runes []rune, from int, d dialect) int {
	s := &scanner{escapeRune: rune(d)}
	for i := from; i < len(runes); i++ {
		s.step(runes[i])
		if s.closed {
			return i
		}
	}
	return len(runes)
}

// diverted takes a rune that is no part of the command line the scanner
// reads itself: a rune of a here-document body, or of a substitution
// inside double quotes, whose end a reader of its own finds; every other
// rune is kept in the line text and answered false.
func (s *scanner) diverted(r rune) bool {
	if s.docs.reading {
		s.bodyRune(r)
		return true
	}
	s.line = append(s.line, r)
	switch {
	case s.inner != nil:
		s.nested(r)
	case s.ticks:
		s.tick(r)
	default:
		return false
	}
	return true
}

// nested takes a rune of a $( ) inside double quotes into the word as it
// stands, while a scanner of its own finds where it ends, the way a POSIX
// shell and PowerShell read it: its quotes, here-document bodies and case
// patterns never end the double quotes around it.
func (s *scanner) nested(r rune) {
	s.word = append(s.word, r)
	s.inner.step(r)
	if s.inner.closed {
		s.inner = nil
	}
}

// tick takes a rune of a backquote substitution inside double quotes into
// the word as it stands: the next backquote a backslash does not escape
// ends it, so a double quote inside it never ends the ones around it.
func (s *scanner) tick(r rune) {
	s.word = append(s.word, r)
	switch {
	case s.escape:
		s.escape = false
	case r == '\\':
		s.escape = true
	case r == '`':
		s.ticks = false
	}
}
