package shell

import "strings"

// segment is one simple command: its words with quotes resolved.
type segment []string

// maxDepth bounds the unwrapping of shells that run a script argument.
const maxDepth = 8

// separators end a segment: command lists, pipelines, subshells and lines.
const separators = ";&|()\n\r"

// segments splits a command line into simple commands: quotes and backslash
// escapes are resolved, the separators end a segment, and a shell that runs
// a script argument (sh -c, bash -lc, pwsh -Command, cmd /c) contributes the
// segments of that script in its place, behind the same assignments and
// wrappers a git call is seen through (env bash -lc, FOO=1 sh -c).
// Expansions stay verbatim: the rules match words, they never run the shell.
func segments(command string) []segment {
	return split(command, 0)
}

func split(command string, depth int) []segment {
	s := &scanner{}
	for _, r := range command {
		s.step(r)
	}
	s.endSegment()
	var out []segment
	for _, seg := range s.out {
		if script, ok := shellScript(segment(unwrap(seg))); ok && depth < maxDepth {
			out = append(out, split(script, depth+1)...)
			continue
		}
		out = append(out, seg)
	}
	return out
}

// scanner reads the command one rune at a time, because a quote, a
// backslash or a separator changes what the next rune means and a split on
// blanks would cut a quoted message at its spaces.
type scanner struct {
	out    []segment
	cur    segment
	word   []rune
	open   bool // a word is being built; an empty quoted string counts
	quote  rune // the active quote, 0 outside quotes
	escape bool // the next rune is taken literally
}

func (s *scanner) step(r rune) {
	switch {
	case s.escape:
		s.escaped(r)
	case s.quote != 0:
		s.quoted(r)
	case r == '\\':
		s.escape, s.open = true, true
	case r == '\'' || r == '"':
		s.quote, s.open = r, true
	case r == ' ' || r == '\t':
		s.endWord()
	case strings.ContainsRune(separators, r):
		s.endSegment()
	default:
		s.open = true
		s.word = append(s.word, r)
	}
}

// escaped takes the rune after a backslash literally; a backslash before a
// newline joins two lines and adds nothing.
func (s *scanner) escaped(r rune) {
	s.escape = false
	if r != '\n' {
		s.word = append(s.word, r)
	}
}

// quoted consumes a rune inside quotes: the closing quote ends them and a
// backslash inside double quotes escapes the next rune.
func (s *scanner) quoted(r rune) {
	switch {
	case r == s.quote:
		s.quote = 0
	case r == '\\' && s.quote == '"':
		s.escape = true
	default:
		s.word = append(s.word, r)
	}
}

func (s *scanner) endWord() {
	if s.open {
		s.cur = append(s.cur, string(s.word))
	}
	s.word, s.open = s.word[:0], false
}

func (s *scanner) endSegment() {
	s.endWord()
	if len(s.cur) > 0 {
		s.out = append(s.out, s.cur)
	}
	s.cur = nil
}

// base is the program name of a word: its last path element, lower case,
// without the .exe suffix, so git.exe on Windows is the same program as git
// for every rule that compares the name.
func base(word string) string {
	return strings.TrimSuffix(strings.ToLower(word[strings.LastIndex(word, "/")+1:]), ".exe")
}

// isShell reports whether a program runs a script given as an argument.
func isShell(name string) bool {
	switch base(name) {
	case "sh", "bash", "zsh", "dash", "ksh", "fish", "pwsh", "powershell", "cmd":
		return true
	default:
		return false
	}
}

// shellScript returns the script a shell segment runs: the words after -c,
// -lc, -Command or /c.
func shellScript(seg segment) (string, bool) {
	if len(seg) < 2 || !isShell(seg[0]) {
		return "", false
	}
	for i := 1; i+1 < len(seg); i++ {
		switch strings.ToLower(seg[i]) {
		case "-c", "-lc", "-ec", "-command", "/c":
			return strings.Join(seg[i+1:], " "), true
		}
	}
	return "", false
}
