package hook

import (
	"strings"
	"unicode"
)

// The strategy options the hook reads from a command text: the two that
// decide which side resolves a conflict hunk, so a merge they made can be
// made again and what it took from a teammate named, never judged.
const (
	optionOurs   = "ours"
	optionTheirs = "theirs"
)

// strategyOptions are the strategy options a command text gives git merge
// or git pull, ours and theirs, in the order it gives them, read only in
// the words of a git merge or git pull the text runs (gitInvocations): the
// word -X or --strategy-option followed by the value, or the value joined
// as -Xtheirs or --strategy-option=theirs, a word read with one pair of
// surrounding quotes. An -X of another program, an option the
// configuration, an alias or a script gives, and any other shell form are
// not read, so the merge the hook makes again then has no option.
func strategyOptions(command string) []string {
	var options []string
	for _, words := range gitInvocations(command) {
		if words[0] != "merge" && words[0] != "pull" {
			continue
		}
		for i, word := range words[1:] {
			value, found := optionValue(bare(word), words[i+2:])
			if value = bare(value); found && (value == optionOurs || value == optionTheirs) {
				options = append(options, value)
			}
		}
	}
	return options
}

// gitInvocations are the git commands a command text runs plainly: for each
// part between two separators of the shell (;, &, | or a line break) whose
// first word is git, its subcommand and the words after it, the global
// options of git before the subcommand skipped with the value of -C and -c.
// The words are read as the shell reads quotes (shellParts), so no word of
// a quoted commit message or value reads as an option. A git a variable, a
// wrapper, an alias or a script runs is not one, a stated limit, because a
// reader widened to them would guess.
func gitInvocations(command string) [][]string {
	var out [][]string
	for _, words := range shellParts(command) {
		if len(words) < 2 || bare(words[0]) != "git" {
			continue
		}
		if rest := subcommand(words[1:]); len(rest) > 0 {
			out = append(out, rest)
		}
	}
	return out
}

// shellParts are the words of each part of a command text between two
// separators of the shell. A span from a quote to the same quote stays in
// one word with its quotes, a separator or a space in it included, a
// backslash outside single quotes keeps the character after it in the word
// and joins two lines before a line break, and a quote left open runs to
// the end of the text, which the shell would refuse to run.
func shellParts(command string) [][]string {
	var scan shellScan
	for _, r := range command {
		scan.next(r)
	}
	scan.endPart()
	return scan.parts
}

// shellScan is what shellParts has read of a command text: the parts it
// ended, the words of the part it reads, the word it reads, the quote of
// the span it is in and whether a backslash escapes the next character.
type shellScan struct {
	parts   [][]string
	words   []string
	word    []rune
	open    bool
	quote   rune
	escaped bool
}

func (s *shellScan) next(r rune) {
	switch {
	case s.escaped:
		s.escaped = false
		if r == '\n' {
			return
		}
		s.write('\\')
	case r == '\\' && s.quote != '\'':
		s.escaped = true
		return
	case s.quote != 0:
		if r == s.quote {
			s.quote = 0
		}
	case r == '\'' || r == '"':
		s.quote = r
	case strings.ContainsRune(";&|\n", r):
		s.endPart()
		return
	case unicode.IsSpace(r):
		s.endWord()
		return
	}
	s.write(r)
}

func (s *shellScan) write(r rune) {
	s.word, s.open = append(s.word, r), true
}

func (s *shellScan) endWord() {
	if s.open {
		s.words, s.word, s.open = append(s.words, string(s.word)), nil, false
	}
}

func (s *shellScan) endPart() {
	s.endWord()
	if len(s.words) > 0 {
		s.parts, s.words = append(s.parts, s.words), nil
	}
}

// subcommand is the subcommand of a git command and the words after it,
// read after the global options, empty when no word names one.
func subcommand(words []string) []string {
	skip := false
	for i, word := range words {
		word = bare(word)
		switch {
		case skip:
			skip = false
		case word == "-C" || word == "-c":
			skip = true
		case strings.HasPrefix(word, "-"):
		default:
			return append([]string{word}, words[i+1:]...)
		}
	}
	return nil
}

// optionValue is the value a word gives the strategy option, from the word
// itself or from the word after it.
func optionValue(word string, after []string) (string, bool) {
	switch {
	case word == "-X" || word == "--strategy-option":
		if len(after) == 0 {
			return "", false
		}
		return after[0], true
	case strings.HasPrefix(word, "--strategy-option="):
		return strings.TrimPrefix(word, "--strategy-option="), true
	case strings.HasPrefix(word, "-X"):
		return strings.TrimPrefix(word, "-X"), true
	default:
		return "", false
	}
}

// bare is a word without the separator of the next command and without one
// pair of surrounding quotes.
func bare(word string) string {
	word = strings.TrimRight(word, ";&|")
	if len(word) >= 2 && (word[0] == '"' || word[0] == '\'') && word[len(word)-1] == word[0] {
		return word[1 : len(word)-1]
	}
	return word
}
