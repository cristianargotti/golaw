package genheader

import (
	"regexp"
	"strings"
)

// conventionLine is the Go convention for a generated file, matched against
// the untrimmed line so an indented copy never counts.
var conventionLine = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

// Convention reports whether content carries the Go convention header
// before its first line that is neither a comment nor blank. It is tighter
// than the class pattern of internal/source, which accepts the line
// anywhere, because a denial must be certain (A47): a hand written file
// that quotes the header in a body or a block comment is never generated.
func Convention(content []byte) bool {
	inBlock := false
	for line := range strings.Lines(string(content)) {
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		trimmed := strings.TrimSpace(line)
		switch {
		case inBlock:
			inBlock = !strings.Contains(line, "*/")
		case trimmed == "":
		case strings.HasPrefix(trimmed, "/*"):
			inBlock = !strings.Contains(trimmed[len("/*"):], "*/")
		case strings.HasPrefix(trimmed, "//"):
			if conventionLine.MatchString(line) {
				return true
			}
		default:
			return false
		}
	}
	return false
}
