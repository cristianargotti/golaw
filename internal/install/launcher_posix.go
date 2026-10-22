package install

import "strings"

// apostrophe closes a single-quoted word, escapes one apostrophe and
// reopens the word, the only way a POSIX shell reads one inside quotes.
const apostrophe = `'\''`

// safeRune reports whether a rune reaches a POSIX shell unchanged when
// bare: letters, digits and the punctuation no shell expands, so the
// words hook and the agent id stay readable in a hooks file.
func safeRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	default:
		return strings.ContainsRune("-_./:=@%+,", r)
	}
}

func safeWord(word string) bool {
	return word != "" && strings.IndexFunc(word, func(r rune) bool { return !safeRune(r) }) < 0
}

// posixQuote writes a word inside single quotes, the one quoting a POSIX
// shell never expands.
func posixQuote(word string) string {
	return "'" + strings.ReplaceAll(word, "'", apostrophe) + "'"
}

// posixCommand writes the executable always quoted, so a path reads as
// one path whatever it holds, and each argument bare when it is safe.
func posixCommand(executable string, args []string) string {
	words := make([]string, 0, len(args)+1)
	words = append(words, posixQuote(executable))
	for _, arg := range args {
		if safeWord(arg) {
			words = append(words, arg)
			continue
		}
		words = append(words, posixQuote(arg))
	}
	return strings.Join(words, " ")
}

// posixWords reads a command back into its words under the grammar
// posixCommand writes: words separated by one space, each bare and safe
// or single-quoted; a shell operator or a second statement has no place
// in it, so such a command is refused here before the regeneration.
func posixWords(command string) ([]string, bool) {
	var words []string
	rest := command
	for {
		word, tail, ok := posixWord(rest)
		if !ok {
			return nil, false
		}
		words = append(words, word)
		if tail == "" {
			return words, true
		}
		if tail[0] != ' ' {
			return nil, false
		}
		rest = tail[1:]
	}
}

// posixWord reads one word and answers the text after it.
func posixWord(text string) (string, string, bool) {
	if text == "" {
		return "", "", false
	}
	if text[0] != '\'' {
		end := strings.IndexFunc(text, func(r rune) bool { return !safeRune(r) })
		switch {
		case end == 0:
			return "", "", false
		case end < 0:
			return text, "", true
		default:
			return text[:end], text[end:], true
		}
	}
	var parts []string
	rest := text[1:]
	for {
		body, after, closed := strings.Cut(rest, "'")
		if !closed {
			return "", "", false
		}
		parts = append(parts, body)
		if !strings.HasPrefix(after, apostrophe[1:]) {
			return strings.Join(parts, "'"), after, true
		}
		rest = after[len(apostrophe)-1:]
	}
}
