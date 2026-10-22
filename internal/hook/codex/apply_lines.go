package codex

import "strings"

// The two line terminators a file holds homogeneously.
const (
	lineFeed = "\n"
	crlf     = "\r\n"
)

// splitLines splits a file into its lines and the terminator they share:
// the line feed, or CRLF when every line feed follows a carriage return. A
// file that mixes the two cannot be rebuilt with one terminator, so it is
// refused rather than rewritten. An unterminated last line is a line and
// an empty file has none.
func splitLines(content []byte) ([]string, string, bool) {
	text := string(content)
	ending := lineFeed
	if pairs := strings.Count(text, crlf); pairs > 0 {
		if pairs != strings.Count(text, lineFeed) {
			return nil, "", false
		}
		ending = crlf
	}
	if text == "" {
		return nil, ending, true
	}
	return strings.Split(strings.TrimSuffix(text, ending), ending), ending, true
}

// joinLines is the file of these lines, each terminated as the host
// terminates every line it writes; no line is an empty file.
func joinLines(lines []string, ending string) []byte {
	if len(lines) == 0 {
		return []byte{}
	}
	return []byte(strings.Join(lines, ending) + ending)
}
