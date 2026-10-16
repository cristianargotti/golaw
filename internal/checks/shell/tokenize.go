package shell

import "strings"

// segment is one simple command, its words with quotes resolved and its
// redirections apart, or one redirection of a command (redirect.go).
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
		segs = scan(command, d)
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

// scan reads a command line of a POSIX shell or of PowerShell into its
// segments, without reading the scripts they run.
func scan(command string, d dialect) []segment {
	s := &scanner{escapeRune: rune(d)}
	for _, r := range command {
		s.step(r)
	}
	s.endInput()
	return s.out
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
}

func (s *scanner) step(r rune) {
	if s.docs.reading {
		s.bodyRune(r)
		return
	}
	if s.amp {
		s.amp = false
		if r == '>' {
			s.closeWord()
			s.op = append(s.op, '&', '>')
			return
		}
		s.endSegment()
	}
	if len(s.op) > 0 && !continuesOperator(s.op, r) {
		s.pending, s.op = string(s.op), s.op[:0]
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

// endWord ends the word being built: the file of the operator waiting for
// one, or the end word of a here-document in a POSIX shell, or a word of
// the command.
func (s *scanner) endWord() {
	switch {
	case s.open && s.pending != "":
		s.redirects = append(s.redirects, segment{s.pending, string(s.word)})
		if doc, strip := heredocOperator(s.pending); doc && s.escapeRune == rune(posix) {
			s.docs.open = append(s.docs.open, heredoc{end: string(s.word), strip: strip})
		}
		s.pending = ""
	case s.open:
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
// -lc, -Command or /c, with the dialect that shell reads it in, and the
// script an eval or an Invoke-Expression reads again (evalScript).
func shellScript(seg segment) (string, dialect, bool) {
	if script, d, ok := evalScript(seg); ok {
		return script, d, true
	}
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

// evalScript returns the words of an eval joined by spaces, which the
// POSIX shell reads again, and the string Invoke-Expression or its alias
// iex runs as PowerShell, after its -Command parameter or an abbreviation
// of it.
func evalScript(seg segment) (string, dialect, bool) {
	switch {
	case len(seg) < 2:
		return "", posix, false
	case seg[0] == "eval":
		return strings.Join(seg[1:], " "), posix, true
	case strings.EqualFold(seg[0], "Invoke-Expression") || strings.EqualFold(seg[0], "iex"):
		args := seg[1:]
		if len(args) > 1 && abbreviates(args[0], "-command") {
			args = args[1:]
		}
		return strings.Join(args, " "), powerShell, true
	default:
		return "", posix, false
	}
}

// abbreviates reports whether a word names a PowerShell parameter by a
// prefix of its name, which PowerShell accepts.
func abbreviates(word, parameter string) bool {
	return len(word) > 1 && strings.HasPrefix(parameter, strings.ToLower(word))
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
