package shell

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
