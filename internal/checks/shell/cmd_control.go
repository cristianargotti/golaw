package shell

import (
	"slices"
	"strings"
)

// spacedComparison is the number of words of an if comparison written
// with spaces: a string, the operator and a string.
const spacedComparison = 3

// keywordFor is the word a for of cmd starts with, in any case.
const keywordFor = "for"

// control ends a whole control prefix as a command of its own and keeps
// the command the set of a for /f runs, which cmd hands to a child cmd /c.
func (l *cmdLine) control() {
	if script, ok := forScript(l.words, string(l.set)); ok {
		l.scripts = append(l.scripts, script)
	}
	l.end()
}

// forScript answers the command the set of a for /f runs: the text
// between single quotes, or between backquotes when its options name
// usebackq, under which a single quoted set is a string.
func forScript(words []string, set string) (string, bool) {
	if len(words) < 2 || !strings.EqualFold(words[0], keywordFor) || !strings.EqualFold(words[1], "/f") {
		return "", false
	}
	quote := "'"
	if slices.ContainsFunc(words, func(word string) bool { return strings.Contains(strings.ToLower(word), "usebackq") }) {
		quote = "`"
	}
	set = strings.TrimSpace(set)
	if len(set) <= len(quote) || !strings.HasPrefix(set, quote) || !strings.HasSuffix(set, quote) {
		return "", false
	}
	return set[1 : len(set)-1], true
}

// cmdControl reports whether the words of a command so far, which hold
// at least the word it starts with, are a whole control prefix: an else,
// the clause of a for up to its do once its set is closed, or an if with
// its condition.
func cmdControl(words []string, sets int) bool {
	switch strings.ToLower(words[0]) {
	case "else":
		return len(words) == 1
	case keywordFor:
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
