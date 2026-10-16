package shell

import "strings"

// The short options of xargs, GNU and BSD together, by what they take: the
// first set takes the rest of its cluster or, at the end of the cluster,
// the next word, the second set an attached value only.
const (
	xargsShortValue    = "adEIJLnPRSs"
	xargsShortOptional = "eil"
)

// xargsCommand answers the command xargs runs, the words after its
// options, or nothing when none follows them: -- ends the options, a long
// option takes the next word when it needs a value not joined by =, and
// a cluster of short options is read letter by letter.
func xargsCommand(words []string) []string {
	for len(words) > 0 {
		word := words[0]
		words = words[1:]
		switch {
		case word == "--":
			return words
		case strings.HasPrefix(word, "--"):
			name, _, joined := strings.Cut(word[2:], "=")
			if !joined && xargsLongValue(name) && len(words) > 0 {
				words = words[1:]
			}
		case len(word) > 1 && word[0] == '-':
			if xargsClusterTakesNext(word[1:]) && len(words) > 0 {
				words = words[1:]
			}
		default:
			return append([]string{word}, words...)
		}
	}
	return nil
}

// xargsClusterTakesNext reports whether a cluster of short options ends
// with one that takes the next word as its value.
func xargsClusterTakesNext(letters string) bool {
	for i, r := range letters {
		switch {
		case strings.ContainsRune(xargsShortValue, r):
			return i == len(letters)-1
		case strings.ContainsRune(xargsShortOptional, r):
			return false
		}
	}
	return false
}

// xargsLongValue reports whether a long option of xargs needs a value.
func xargsLongValue(name string) bool {
	switch name {
	case "arg-file", "delimiter", "max-args", "max-procs", "max-chars", "process-slot-var":
		return true
	default:
		return false
	}
}
