package shell

import (
	"slices"
	"strings"
)

// cmdSpecial are the runes whose presence between the two quotes of a /c
// script makes cmd strip them.
const cmdSpecial = "&<>()@^|"

// cmdDelimiters are the runes cmd skips before the name of a command.
const cmdDelimiters = " \t,;="

// spacedComparison is the number of words of an if comparison written
// with spaces: a string, the operator and a string.
const spacedComparison = 3

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
	cur    []rune   // the command being read, as its program receives it
	word   []rune   // the word of that command being read, its quotes kept
	words  []string // the words of that command read so far
	quoted bool     // inside cmd's double quotes, which a backslash never escapes
	caret  bool     // the next rune is literal
	blocks int      // the parenthesized blocks open
	sets   int      // the parentheses the set of a for holds open
}

// cmdCommands splits a script into the commands cmd runs: outside its
// quotes a caret makes the next rune literal and is dropped, an ampersand,
// a pipe or a line end separates two commands, a parenthesis opens a block
// at the start of a command or closes an open one, the delimiters and an
// at sign before a command are dropped, a semicolon inside one separates
// nothing, and the condition of an if, the clause of a for up to its do
// and an else are no part of the command they run (cmdControl), because
// cmd reads them and runs what follows.
func cmdCommands(script string) []string {
	l := &cmdLine{}
	for _, r := range script {
		l.step(r)
	}
	l.end()
	return l.out
}

func (l *cmdLine) step(r rune) {
	switch {
	case l.caret:
		l.caret = false
		if r != '\n' && r != '\r' {
			l.add(r)
		}
	case r == '"':
		l.quoted = !l.quoted
		l.add(r)
	case l.quoted:
		l.add(r)
	case r == '^':
		l.caret = true
	case r == '&' || r == '|' || r == '\n' || r == '\r':
		l.end()
	default:
		l.plain(r)
	}
}

// plain reads a rune outside quotes that separates no command.
func (l *cmdLine) plain(r rune) {
	switch {
	case len(l.cur) == 0 && (r == '@' || strings.ContainsRune(cmdDelimiters, r)):
	case r == ' ' || r == '\t':
		l.boundary(r)
	case r == '(':
		l.open()
	case r == ')' && l.sets > 0:
		l.sets--
		l.add(r)
	case r == ')' && l.blocks > 0:
		l.blocks--
		l.end()
	default:
		l.add(r)
	}
}

// boundary reads white space outside quotes: it ends a word, and a word
// that completes a control prefix drops the prefix, so the command the
// prefix runs starts after it.
func (l *cmdLine) boundary(r rune) {
	if len(l.word) > 0 {
		l.words = append(l.words, string(l.word))
		l.word = l.word[:0]
	}
	if cmdControl(l.words, l.sets) {
		l.restart()
		return
	}
	l.cur = append(l.cur, r)
}

// open reads a parenthesis outside quotes: it opens a block at the start
// of a command or right after a control prefix, belongs to the set of a
// for before its do, and is an ordinary rune anywhere else.
func (l *cmdLine) open() {
	words := l.words
	if len(l.word) > 0 {
		words = append(slices.Clip(words), string(l.word))
	}
	switch {
	case len(l.cur) == 0 || cmdControl(words, l.sets):
		l.restart()
		l.blocks++
	case len(words) > 0 && strings.EqualFold(words[0], "for"):
		l.sets++
		l.add('(')
	default:
		l.add('(')
	}
}

func (l *cmdLine) add(r rune) {
	l.cur = append(l.cur, r)
	l.word = append(l.word, r)
}

func (l *cmdLine) end() {
	l.out = append(l.out, string(l.cur))
	l.restart()
}

// restart begins another command in the blocks open.
func (l *cmdLine) restart() {
	l.cur, l.word, l.words, l.sets = l.cur[:0], l.word[:0], nil, 0
}

// cmdControl reports whether the words of a command so far, which hold
// at least the word it starts with, are a whole control prefix: an else,
// the clause of a for up to its do once its set is closed, or an if with
// its condition.
func cmdControl(words []string, sets int) bool {
	switch strings.ToLower(words[0]) {
	case "else":
		return len(words) == 1
	case "for":
		return len(words) > 1 && sets == 0 && strings.EqualFold(words[len(words)-1], "do")
	case "if":
		return ifCondition(words[1:])
	default:
		return false
	}
}

// ifCondition reports whether the words are the whole condition of an if:
// /i and not before it, then a test that names one word (exist, defined,
// errorlevel, cmdextversion) or two strings compared by == or by an
// operator, spaced or not.
func ifCondition(words []string) bool {
	for len(words) > 0 && (strings.EqualFold(words[0], "/i") || strings.EqualFold(words[0], "not")) {
		words = words[1:]
	}
	switch len(words) {
	case 1:
		at := strings.Index(words[0], "==")
		return at > 0 && at+2 < len(words[0])
	case 2:
		return oneWordTest(words[0]) ||
			len(words[0]) > 2 && strings.HasSuffix(words[0], "==") ||
			len(words[1]) > 2 && strings.HasPrefix(words[1], "==")
	case spacedComparison:
		return compareOperator(words[1])
	default:
		return false
	}
}

// oneWordTest reports whether an if test takes the one word after it.
func oneWordTest(word string) bool {
	switch strings.ToLower(word) {
	case "exist", "defined", "errorlevel", "cmdextversion":
		return true
	default:
		return false
	}
}

// compareOperator reports whether a word compares the two strings of an if.
func compareOperator(word string) bool {
	switch strings.ToLower(word) {
	case "==", "equ", "neq", "lss", "leq", "gtr", "geq":
		return true
	default:
		return false
	}
}
