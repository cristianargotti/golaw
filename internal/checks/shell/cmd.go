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

// splitCmd reads a script cmd runs (cmd /c) in two passes, because cmd and
// the program it starts read the line apart: cmd applies the quote rule of
// /c and separates the commands by its own quoting, then the program splits
// its arguments by the Microsoft runtime rules. The two runtimes in use
// read a doubled quote inside quotes differently, so a command whose two
// readings differ contributes both, and the call and start of cmd add
// the command they run (cmdSegments).
func splitCmd(script string) []segment {
	var out []segment
	for _, reading := range cmdReadings(script) {
		commands, scripts := cmdCommands(reading)
		for _, command := range commands {
			out = append(out, cmdSegments(command)...)
		}
		for _, inner := range scripts {
			out = append(out, segment{cmdProgram, "/c", inner})
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
	out       []string
	cur       []rune   // the command being read, as its program receives it
	word      []rune   // the word of that command being read, its quotes kept
	words     []string // the words of that command read so far
	quoted    bool     // inside cmd's double quotes, which a backslash never escapes
	caret     bool     // the next rune is literal
	blocks    int      // the parenthesized blocks open
	sets      int      // the parentheses the set of a for holds open
	set       []rune   // the text of that set, which a for /f may run
	scripts   []string // the commands the sets of a for /f run
	op        []rune   // the redirection operator being read
	file      []rune   // the file of that redirection
	inFile    bool     // the operator is whole and its file is being read
	redirects []string // the redirections of the command, each its operator and file
}

// cmdCommands splits a script into the commands cmd runs: outside its
// quotes a caret makes the next rune literal and is dropped, an ampersand,
// a pipe or a line end separates two commands, a parenthesis opens a block
// at the start of a command or closes an open one, the delimiters and an
// at sign before a command are dropped, a semicolon inside one separates
// nothing, and the condition of an if, the clause of a for up to its do
// and an else are no part of the command they run (cmdControl), because
// cmd reads them and runs what follows; such a prefix is a command of its
// own, because the names it holds, the set of a for or the file of an if
// exist, are read by what it runs, and the items of a set are apart from
// its parentheses and its delimiters. The commands the sets of a for /f
// run are answered apart, as the scripts of a child cmd /c.
func cmdCommands(script string) ([]string, []string) {
	l := &cmdLine{}
	for _, r := range script {
		l.step(r)
	}
	l.end()
	return l.out, l.scripts
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
	case l.redirection(r):
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
	case l.sets > 0 && strings.ContainsRune(cmdDelimiters, r):
		l.item(r)
	case r == ' ' || r == '\t':
		l.boundary(r)
	case r == '(':
		l.open()
	case r == ')' && l.sets > 0:
		l.sets--
		l.item(r)
	case r == ')' && l.blocks > 0:
		l.blocks--
		l.end()
	default:
		l.add(r)
	}
}

// boundary reads white space outside quotes: it ends a word, and a word
// that completes a control prefix ends the prefix, so the command the
// prefix runs starts after it.
func (l *cmdLine) boundary(r rune) {
	if len(l.word) > 0 {
		l.words = append(l.words, string(l.word))
		l.word = l.word[:0]
	}
	if cmdControl(l.words, l.sets) {
		l.control()
		return
	}
	l.cur = append(l.cur, r)
}

// item ends an item of the set of a for at a delimiter or a parenthesis of
// the set, which stays in the text of the set while it is open: the runes
// of the item stay in the command for the rules that read names and never
// count as a word of the for, so an item named do completes no prefix.
func (l *cmdLine) item(r rune) {
	if l.sets > 0 {
		l.set = append(l.set, r)
	}
	l.word = l.word[:0]
	l.cur = append(l.cur, ' ')
}

// open reads a parenthesis outside quotes: it opens a block at the start
// of a command or right after a control prefix, opens the set of a for
// before its do, nests inside a set, and is an ordinary rune anywhere
// else.
func (l *cmdLine) open() {
	words := l.words
	if len(l.word) > 0 {
		words = append(slices.Clip(words), string(l.word))
	}
	switch {
	case l.sets > 0:
		l.item('(')
		l.sets++
	case len(l.cur) == 0 || cmdControl(words, l.sets):
		l.control()
		l.blocks++
	case len(words) > 0 && strings.EqualFold(words[0], keywordFor):
		l.words, l.set = words, l.set[:0]
		l.item('(')
		l.sets++
	default:
		l.add('(')
	}
}

// add takes a rune of the command, or of the file of the redirection
// being read.
func (l *cmdLine) add(r rune) {
	if len(l.op) > 0 {
		l.inFile = true
		l.file = append(l.file, r)
		return
	}
	if l.sets > 0 {
		l.set = append(l.set, r)
	}
	l.cur = append(l.cur, r)
	l.word = append(l.word, r)
}

// end ends the command being read, then its redirections, each a command
// of its own.
func (l *cmdLine) end() {
	l.flush()
	l.out = append(l.out, string(l.cur))
	l.out = append(l.out, l.redirects...)
	l.redirects = l.redirects[:0]
	l.restart()
}

// restart begins another command in the blocks open.
func (l *cmdLine) restart() {
	l.cur, l.word, l.words, l.sets, l.set = l.cur[:0], l.word[:0], nil, 0, l.set[:0]
}
