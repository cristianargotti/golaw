package shell

import (
	"regexp"
	"strings"
)

// posixStream is a word that names the stream an operator right after it
// redirects in a POSIX shell: a file descriptor or a {name} to assign one.
var posixStream = regexp.MustCompile(`^(?:\d+|\{[A-Za-z_][A-Za-z0-9_]*\})$`)

// The redirections of a command leave the words its program receives
// before it runs, wherever they stand, so each one is a segment of its
// own, its operator and its file, which no rule reads as a program and
// the secret rule judges by name.

// operatorRune reads a rune after an operator rune: after an ampersand a
// greater-than sign makes the operator &>, which takes the rune, and any
// other rune leaves the ampersand to separate the commands; an operator
// the rune does not extend is whole, its file the next word.
func (s *scanner) operatorRune(r rune) bool {
	if s.amp {
		s.amp = false
		if r == '>' {
			s.closeWord()
			s.op = append(s.op, '&', '>')
			return true
		}
		s.endSegment()
	}
	if len(s.op) > 0 && !continuesOperator(s.op, r) {
		s.pending, s.op = string(s.op), s.op[:0]
	}
	return false
}

// redirect reads a rune of a redirection operator outside quotes. The
// first one starts the operator: a word right before it that names a
// stream joins it, and any other word ends there, since an operator splits
// a word.
func (s *scanner) redirect(r rune) {
	if len(s.op) == 0 {
		if s.open && !s.literal && s.pending == "" && s.stream(string(s.word)) {
			s.op = append(s.op, s.word...)
			s.word, s.open = s.word[:0], false
		} else {
			s.closeWord()
		}
	}
	s.op = append(s.op, r)
}

// stream reports whether a word right before an operator names the stream
// it redirects: digits, a {name} in a POSIX shell, and in PowerShell the
// star of every stream, which a POSIX shell reads as a pattern.
func (s *scanner) stream(word string) bool {
	if s.escapeRune == rune(powerShell) {
		return word == "*" || word != "" && strings.Trim(word, "0123456789") == ""
	}
	return posixStream.MatchString(word)
}

// closeWord ends the word before an operator: the file of the operator
// before it, a word of the command, or nothing, when an operator still
// waits for its file and is kept without one.
func (s *scanner) closeWord() {
	if !s.open && s.pending != "" {
		s.redirects = append(s.redirects, segment{s.pending})
		s.pending = ""
	}
	s.endWord()
}

// continuesOperator reports whether a rune extends the operator read so
// far: another angle bracket (>>, <<, <<<, <>), an ampersand after one
// (>&, <&), a pipe after a greater-than sign (>|) or the dash of <<-.
func continuesOperator(op []rune, r rune) bool {
	last := op[len(op)-1]
	switch r {
	case '<', '>':
		return true
	case '&':
		return last == '<' || last == '>'
	case '|':
		return last == '>'
	case '-':
		return strings.HasSuffix(string(op), "<<") && !strings.HasSuffix(string(op), "<<<")
	default:
		return false
	}
}

// redirection reads a rune outside quotes and carets for the redirections
// of a cmd command and reports whether it took it: cmd removes a
// redirection, a handle digit right before it, its operator and its file,
// from the command its program receives, wherever it stands, inside a word
// too; white space, a separator or a closing parenthesis ends the file.
// Inside the set of a for, which cmd wants escaped, an operator stays in
// the set, so the command a for /f runs keeps its own redirections.
func (l *cmdLine) redirection(r rune) bool {
	switch {
	case len(l.op) > 0 && !l.inFile && (r == '<' || r == '>' || r == '&' && strings.ContainsRune("<>", l.op[len(l.op)-1])):
		l.op = append(l.op, r)
	case (r == '<' || r == '>') && l.sets == 0:
		l.flush()
		l.operator(r)
	case len(l.op) == 0:
		return false
	case r == ' ' || r == '\t':
		if len(l.file) > 0 {
			l.flush()
			return false
		}
		l.inFile = true
	case r == '&' || r == '|' || r == '\n' || r == '\r' || r == ')' && (l.blocks > 0 || l.sets > 0):
		l.flush()
		return false
	default:
		l.inFile = true
		l.file = append(l.file, r)
	}
	return true
}

// operator begins a redirection of cmd: a handle digit right before the
// operator, a word of its own, belongs to it, and any other word ends
// there.
func (l *cmdLine) operator(r rune) {
	if len(l.word) == 1 && l.word[0] >= '0' && l.word[0] <= '9' {
		l.op = append(l.op, l.word[0])
		l.cur, l.word = l.cur[:len(l.cur)-1], l.word[:0]
	} else if len(l.word) > 0 {
		l.words = append(l.words, string(l.word))
		l.word = l.word[:0]
	}
	l.op = append(l.op, r)
}

// flush ends the redirection being read, if any, as a command of its own:
// its operator, then its file.
func (l *cmdLine) flush() {
	if len(l.op) > 0 {
		l.redirects = append(l.redirects, string(l.op)+" "+string(l.file))
	}
	l.op, l.file, l.inFile = l.op[:0], l.file[:0], false
}
