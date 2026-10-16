package shell

import (
	"slices"
	"strings"
)

// heredoc is a here-document a line of a POSIX shell opens: the word that
// ends its body, whether the body may indent that word with tabs, and
// whether the word was quoted, which keeps the shell from expanding the
// body.
type heredoc struct {
	end    string
	strip  bool
	quoted bool
}

// documents holds the here-documents of a line and the lines after it,
// which are their bodies: the stdin of the command that opened them, so
// data, unless a word of that line names a shell, which then reads a body
// as its script.
type documents struct {
	open    []heredoc // the documents the line opened, read after it in order
	reading bool      // the lines being read are their bodies
	shell   string    // the shell a word of the line names, empty for none
	first   int       // the index in out of the first segment of the line
	line    []rune    // the body line being read
	body    []rune    // the body read so far of the first document open
}

// heredocOperator reports whether a redirection operator opens a
// here-document, << or <<-, a stream before it or not, and whether its
// body may indent the end word with tabs; <<< is a here-string.
func heredocOperator(op string) (bool, bool) {
	switch {
	case strings.HasSuffix(op, "<<-"):
		return true, true
	case strings.HasSuffix(op, "<<") && !strings.HasSuffix(op, "<<<"):
		return true, false
	default:
		return false, false
	}
}

// newLine ends a line of the command: the documents it opened, if any,
// read the lines after it as their bodies.
func (s *scanner) newLine() {
	if len(s.docs.open) == 0 {
		s.docs.first = len(s.out)
		return
	}
	s.docs.reading, s.docs.shell = true, ""
	for _, seg := range s.out[s.docs.first:] {
		if i := slices.IndexFunc(seg, isShell); i >= 0 {
			s.docs.shell = seg[i]
			break
		}
	}
}

// bodyRune reads a rune of the bodies, a line at a time.
func (s *scanner) bodyRune(r rune) {
	if r != '\n' {
		s.docs.line = append(s.docs.line, r)
		return
	}
	s.bodyLine()
}

// bodyLine ends a line of the bodies: a line equal to the end word of the
// first document open, its tabs stripped under <<-, ends its body.
func (s *scanner) bodyLine() {
	doc := s.docs.open[0]
	line := strings.TrimSuffix(string(s.docs.line), "\r")
	if doc.strip {
		line = strings.TrimLeft(line, "\t")
	}
	if line == doc.end {
		s.closeDocument()
	} else {
		s.docs.body = append(append(s.docs.body, s.docs.line...), '\n')
	}
	s.docs.line = s.docs.line[:0]
}

// closeDocument ends the body of the first document open, which the shell
// its line names reads as a script of its own and which the shell expands
// when its end word is unquoted.
func (s *scanner) closeDocument() {
	if s.docs.shell != "" {
		s.out = append(s.out, segment{s.docs.shell, "-c", string(s.docs.body)})
	}
	if !s.docs.open[0].quoted {
		s.bodies = append(s.bodies, string(s.docs.body))
	}
	s.docs.body, s.docs.open = s.docs.body[:0], s.docs.open[1:]
	if len(s.docs.open) == 0 {
		s.docs.reading, s.docs.first = false, len(s.out)
	}
}

// endInput ends the command line: a body line without its newline is
// read, and a body the line never ended stands as it is.
func (s *scanner) endInput() {
	if s.docs.reading && len(s.docs.line) > 0 {
		s.bodyLine()
	}
	for s.docs.reading {
		s.closeDocument()
	}
	s.endSegment()
}
