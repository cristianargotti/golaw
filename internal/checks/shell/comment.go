package shell

// comment is the kind of comment the scanner reads: a line comment, which
// a hash that starts a word opens in a POSIX shell and in PowerShell, or a
// PowerShell block comment, from <# to #>.
type comment int

const (
	noComment comment = iota
	lineComment
	blockComment
)

// commented reads a rune of a comment, which no rule reads and the shell
// never expands, and reports whether it took it: a line comment ends
// before its newline, which ends the line as any other, and a block
// comment with its #>.
func (s *scanner) commented(r rune) bool {
	switch {
	case s.comment == lineComment && r == '\n':
		s.comment = noComment
		return false
	case s.comment == blockComment && s.hash && r == '>':
		s.comment = noComment
	}
	s.hash = r == '#'
	return true
}
