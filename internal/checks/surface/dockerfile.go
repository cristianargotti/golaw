package surface

import (
	"bytes"
	"strings"
)

// instruction is one Dockerfile instruction: its keyword upper-cased, its
// arguments with continuation lines joined by one space, and the 1-based
// line it starts on.
type instruction struct {
	keyword string
	args    string
	line    int
}

// readInstructions answers the instructions of a Dockerfile. A comment
// line, a parser directive included, is skipped inside a continuation too,
// as the builder skips it. Smallest behavior: the escape directive is not
// read, so the backslash is the only continuation, which every Dockerfile
// the kit has met uses.
func readInstructions(src []byte) []instruction {
	var out []instruction
	var current *instruction
	for i, raw := range bytes.Split(src, []byte("\n")) {
		text := strings.TrimSpace(strings.TrimSuffix(string(raw), "\r"))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		body, continued := strings.CutSuffix(text, `\`)
		body = strings.TrimSpace(body)
		if current == nil {
			keyword, args := splitWord(body)
			out = append(out, instruction{keyword: strings.ToUpper(keyword), args: args, line: i + 1})
			current = &out[len(out)-1]
		} else if body != "" {
			current.args = strings.TrimSpace(current.args + " " + body)
		}
		if !continued {
			current = nil
		}
	}
	return out
}

// splitWord answers the first word of a line and the rest, trimmed; a
// space or a tab ends the word.
func splitWord(text string) (string, string) {
	end := strings.IndexAny(text, " \t")
	if end < 0 {
		return text, ""
	}
	return text[:end], strings.TrimSpace(text[end:])
}
