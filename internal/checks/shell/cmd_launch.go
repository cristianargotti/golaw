package shell

import (
	"slices"
	"strings"
)

// cmdSegments answers the segments one command of a cmd script
// contributes: its words by both runtime readings and, for each, the
// commands its call and start run, each segment once.
func cmdSegments(command string) []segment {
	var out []segment
	for _, legacy := range []bool{false, true} {
		words := msvcWords(command, legacy)
		for _, run := range append([][]string{words}, cmdLaunched(words)...) {
			if len(run) > 0 && !slices.ContainsFunc(out, func(seg segment) bool { return slices.Equal(seg, run) }) {
				out = append(out, run)
			}
		}
	}
	return out
}

// cmdLaunched answers the commands the call and start of cmd run: call
// runs the rest of its line, and start the program after its switches.
// The first word after the switches is the program or, when it was quoted,
// a title before more switches and the program; the words no longer carry
// their quotes, so both readings are answered. Every such command is the
// tail of the words from some index, so each index is read once: a run of
// start words then costs one reading per word instead of doubling at each.
func cmdLaunched(words []string) [][]string {
	var out [][]string
	seen := make([]bool, len(words)+1)
	for queue := []int{0}; len(queue) > 0; queue = queue[1:] {
		for _, at := range launches(words, queue[0]) {
			if !seen[at] {
				seen[at] = true
				out = append(out, words[at:])
				queue = append(queue, at)
			}
		}
	}
	return out
}

// launches answers the indices of the commands the call or the start at
// an index runs: the word after call, and the program after the switches
// of start with, when a word follows it, the program after a title.
func launches(words []string, at int) []int {
	if len(words)-at < 2 {
		return nil
	}
	switch strings.ToLower(words[at]) {
	case "call":
		return []int{at + 1}
	case "start":
		rest := startSwitches(words[at+1:])
		if len(rest) == 0 {
			return nil
		}
		if titled := startSwitches(rest[1:]); len(titled) > 0 {
			return []int{len(words) - len(rest), len(words) - len(titled)}
		}
		return []int{len(words) - len(rest)}
	default:
		return nil
	}
}

// startSwitches drops the switches start reads before its program: /d,
// /node, /affinity and /machine take the next word as their value, and any
// other word that starts with a slash is a switch of its own.
func startSwitches(words []string) []string {
	for len(words) > 0 && strings.HasPrefix(words[0], "/") {
		switch strings.ToLower(words[0]) {
		case "/d", "/node", "/affinity", "/machine":
			if len(words) < 2 {
				return nil
			}
			words = words[2:]
		default:
			words = words[1:]
		}
	}
	return words
}
