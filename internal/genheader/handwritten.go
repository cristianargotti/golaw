package genheader

import "strings"

// Handwritten reads the escape of M7.9.5 from the leading comments: the
// reason after the marker, empty for a bare marker, and whether the file
// is marked at all.
func Handwritten(content []byte) (string, bool) {
	_, reason, marked := HandwrittenAt(content)
	return reason, marked
}

// HandwrittenAt answers the 1-based line of the marker, the reason and the
// mark, or zero values for an unmarked file. Only line comments before the
// first line that is neither blank nor a line comment count, and a block
// comment ends that section: the smallest reading of "leading comments",
// so a marker quoted in a body or after the package clause never turns a
// file into an escape.
func HandwrittenAt(content []byte) (int, string, bool) {
	number := 0
	for text := range strings.Lines(string(content)) {
		number++
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		comment, ok := strings.CutPrefix(trimmed, "//")
		if !ok {
			return 0, "", false
		}
		fields := strings.Fields(comment)
		if len(fields) > 0 && fields[0] == HandwrittenMarker {
			rest, _ := strings.CutPrefix(strings.TrimSpace(comment), HandwrittenMarker)
			return number, strings.TrimSpace(rest), true
		}
	}
	return 0, "", false
}
