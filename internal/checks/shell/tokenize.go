package shell

import "strings"

// segment is one simple command: its words with quotes resolved.
type segment []string

// maxDepth bounds the unwrapping of shells that run a script argument.
const maxDepth = 8

// separators end a segment: command lists, pipelines, subshells and lines.
const separators = ";&|()\n\r"

// dialect is how a shell reads its command line, named by the rune that
// escapes the next one outside quotes and inside double quotes.
type dialect rune

// The dialects of the tools and the wrappers: a POSIX shell escapes with a
// backslash, PowerShell with a backtick, where the backslash is the path
// separator, and cmd is read apart (splitCmd), by its own rules and by the
// Microsoft argument rules of the program it starts.
const (
	posix      dialect = '\\'
	powerShell dialect = '`'
	cmdShell   dialect = 0
)

// cmdProgram is the program name of cmd, which reads its script apart.
const cmdProgram = "cmd"

// segments splits a command line into simple commands the way the tool's
// shell reads it: quotes and escapes are resolved, the separators end a
// segment, and a shell that runs a script argument (sh -c, bash -lc, pwsh
// -Command, cmd /c) contributes the segments of that script, read in its
// own dialect, in its place, behind the same assignments and wrappers a
// git call is seen through (env bash -lc, FOO=1 sh -c). Expansions stay
// verbatim: the rules match words, they never run the shell.
func segments(command, tool string) []segment {
	if strings.EqualFold(tool, "PowerShell") {
		return split(command, powerShell, 0)
	}
	return split(command, posix, 0)
}

func split(command string, d dialect, depth int) []segment {
	var segs []segment
	if d == cmdShell {
		segs = splitCmd(command)
	} else {
		s := &scanner{escapeRune: rune(d)}
		for _, r := range command {
			s.step(r)
		}
		s.endSegment()
		segs = s.out
	}
	var out []segment
	for _, seg := range segs {
		if script, inner, ok := shellScript(segment(unwrap(seg))); ok && depth < maxDepth {
			out = append(out, split(script, inner, depth+1)...)
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
	out        []segment
	cur        segment
	word       []rune
	open       bool // a word is being built; an empty quoted string counts
	quote      rune // the active quote, 0 outside quotes
	escape     bool // the next rune is taken literally
	escapeRune rune // the escape of the dialect, 0 for none
}

func (s *scanner) step(r rune) {
	switch {
	case s.escape:
		s.escaped(r)
	case s.quote != 0:
		s.quoted(r)
	case s.escapeRune != 0 && r == s.escapeRune:
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

// quoted consumes a rune inside quotes: the closing quote ends them and the
// escape of the dialect inside double quotes escapes the next rune.
func (s *scanner) quoted(r rune) {
	switch {
	case r == s.quote:
		s.quote = 0
	case s.escapeRune != 0 && r == s.escapeRune && s.quote == '"':
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

// base is the program name of a word: its last path element after a slash
// or a backslash, lower case, without the .exe suffix, so C:\Git\cmd\git.exe
// on Windows is the same program as git for every rule that compares the
// name.
func base(word string) string {
	return strings.TrimSuffix(strings.ToLower(word[strings.LastIndexAny(word, `/\`)+1:]), ".exe")
}

// isShell reports whether a program runs a script given as an argument.
func isShell(name string) bool {
	switch base(name) {
	case "sh", "bash", "zsh", "dash", "ksh", "fish", "pwsh", "powershell", cmdProgram:
		return true
	default:
		return false
	}
}

// shellScript returns the script a shell segment runs, the words after -c,
// -lc, -Command or /c, with the dialect that shell reads it in.
func shellScript(seg segment) (string, dialect, bool) {
	if len(seg) < 2 || !isShell(seg[0]) {
		return "", posix, false
	}
	for i := 1; i+1 < len(seg); i++ {
		switch strings.ToLower(seg[i]) {
		case "-c", "-lc", "-ec", "-command", "/c":
			return strings.Join(seg[i+1:], " "), dialectOf(seg[0]), true
		}
	}
	return "", posix, false
}

// dialectOf is the dialect of a shell program.
func dialectOf(program string) dialect {
	switch base(program) {
	case "pwsh", "powershell":
		return powerShell
	case cmdProgram:
		return cmdShell
	default:
		return posix
	}
}
