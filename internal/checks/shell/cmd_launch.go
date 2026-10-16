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
// their quotes, so both readings are answered.
func cmdLaunched(words []string) [][]string {
	if len(words) < 2 {
		return nil
	}
	switch strings.ToLower(words[0]) {
	case "call":
		return append([][]string{words[1:]}, cmdLaunched(words[1:])...)
	case "start":
		rest := startSwitches(words[1:])
		if len(rest) == 0 {
			return nil
		}
		out := append([][]string{rest}, cmdLaunched(rest)...)
		if titled := startSwitches(rest[1:]); len(titled) > 0 {
			out = append(append(out, titled), cmdLaunched(titled)...)
		}
		return out
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
