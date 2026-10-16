package shell

import (
	"slices"
	"strings"
)

// cmdSpecial are the runes whose presence between the two quotes of a /c
// script makes cmd strip them.
const cmdSpecial = "&<>()@^|"

// splitCmd reads a script cmd runs (cmd /c) in two passes, because cmd and
// the program it starts read the line apart: cmd applies the quote rule of
// /c and separates the commands by its own quoting, then the program splits
// its arguments by the Microsoft runtime rules. The two runtimes in use
// read a doubled quote inside quotes differently, so a command whose two
// readings differ contributes both.
func splitCmd(script string) []segment {
	var out []segment
	for _, reading := range cmdReadings(script) {
		for _, command := range cmdCommands(reading) {
			modern, legacy := msvcWords(command, false), msvcWords(command, true)
			if len(modern) > 0 {
				out = append(out, modern)
			}
			if len(legacy) > 0 && !slices.Equal(modern, legacy) {
				out = append(out, legacy)
			}
		}
	}
	return out
}

// cmdReadings applies the quote rule of cmd /c: a script that starts with
// a quote loses it and the last quote of the line, unless the script holds
// exactly two, with white space and no special rune between them, which
// cmd keeps when they name an executable; that last test needs the disk,
// so both readings are judged.
func cmdReadings(script string) []string {
	trimmed := strings.TrimLeft(script, " \t")
	if !strings.HasPrefix(trimmed, `"`) {
		return []string{script}
	}
	last := strings.LastIndex(trimmed, `"`)
	if last == 0 {
		return []string{trimmed[1:]}
	}
	inner := trimmed[1:last]
	stripped := inner + trimmed[last+1:]
	if strings.Count(trimmed, `"`) == 2 && !strings.ContainsAny(inner, cmdSpecial) && strings.ContainsAny(inner, " \t") {
		return []string{trimmed, stripped}
	}
	return []string{stripped}
}

// cmdLine reads a script rune by rune the way cmd finds its commands.
type cmdLine struct {
	out    []string
	cur    []rune
	quoted bool // inside cmd's double quotes, which a backslash never escapes
	caret  bool // the next rune is literal
	blocks int  // the parenthesized blocks open
}

// cmdCommands splits a script into the commands cmd runs: outside its
// quotes a caret makes the next rune literal and is dropped, an ampersand,
// a pipe or a line end separates two commands, a parenthesis opens a block
// at the start of a command or closes an open one, and a leading at sign
// is dropped; a semicolon separates nothing.
func cmdCommands(script string) []string {
	l := &cmdLine{}
	for _, r := range script {
		l.step(r)
	}
	l.end()
	return l.out
}

func (l *cmdLine) step(r rune) {
	start := strings.TrimLeft(string(l.cur), " \t") == ""
	switch {
	case l.caret:
		l.caret = false
		if r != '\n' && r != '\r' {
			l.cur = append(l.cur, r)
		}
	case r == '"':
		l.quoted = !l.quoted
		l.cur = append(l.cur, r)
	case l.quoted:
		l.cur = append(l.cur, r)
	case r == '^':
		l.caret = true
	case r == '&' || r == '|' || r == '\n' || r == '\r':
		l.end()
	case r == '(' && start:
		l.blocks++
	case r == ')' && l.blocks > 0:
		l.blocks--
		l.end()
	case r == '@' && start:
	default:
		l.cur = append(l.cur, r)
	}
}

func (l *cmdLine) end() {
	l.out = append(l.out, string(l.cur))
	l.cur = l.cur[:0]
}

// msvcArgs splits one command into the arguments the Microsoft runtime
// gives its program.
type msvcArgs struct {
	words   []string
	word    []rune
	open    bool // a word is being built; an empty quoted string counts
	quoted  bool
	slashes int  // the backslashes read and not yet placed
	legacy  bool // a doubled quote inside quotes also ends them, as msvcrt reads it
}

// msvcWords splits a command into its arguments: white space separates
// them outside double quotes, a run of backslashes before a double quote
// keeps one backslash per pair and, when it is odd, makes the quote
// literal, a run before any other rune stays as it is, two double quotes
// inside quotes are one literal quote, and a single quote is an ordinary
// rune.
func msvcWords(command string, legacy bool) []string {
	a := &msvcArgs{legacy: legacy}
	runes := []rune(command)
	for i := 0; i < len(runes); i++ {
		switch r := runes[i]; r {
		case '\\':
			a.slashes++
			a.open = true
		case '"':
			if a.quote(i+1 < len(runes) && runes[i+1] == '"') {
				i++
			}
		default:
			a.other(r)
		}
	}
	a.place(a.slashes)
	a.endWord()
	return a.words
}

// quote reads a double quote after the backslashes before it and reports
// whether it took the next quote with it.
func (a *msvcArgs) quote(doubled bool) bool {
	odd := a.slashes%2 == 1
	a.place(a.slashes / 2)
	a.open = true
	switch {
	case odd:
		a.word = append(a.word, '"')
	case a.quoted && doubled:
		a.word = append(a.word, '"')
		a.quoted = !a.legacy
		return true
	default:
		a.quoted = !a.quoted
	}
	return false
}

// other reads any rune but a backslash or a quote: the backslashes before
// it stay as they are, and white space outside quotes ends the word.
func (a *msvcArgs) other(r rune) {
	a.place(a.slashes)
	if (r == ' ' || r == '\t') && !a.quoted {
		a.endWord()
		return
	}
	a.word = append(a.word, r)
	a.open = true
}

func (a *msvcArgs) place(n int) {
	for range n {
		a.word = append(a.word, '\\')
	}
	a.slashes = 0
}

func (a *msvcArgs) endWord() {
	if a.open {
		a.words = append(a.words, string(a.word))
	}
	a.word, a.open = a.word[:0], false
}
