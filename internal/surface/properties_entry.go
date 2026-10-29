package surface

import "strings"

// splitEntry splits a logical line as Java's loader does: the key ends at
// the first =, : or blank no backslash escapes, and the separator is the
// blanks around one optional = or :.
func splitEntry(line string) (string, string) {
	end := len(line)
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' {
			i++
			continue
		}
		if line[i] == '=' || line[i] == ':' || blank(line[i]) {
			end = i
			break
		}
	}
	rest := strings.TrimLeft(line[end:], " \t\f")
	if rest != "" && (rest[0] == '=' || rest[0] == ':') {
		rest = strings.TrimLeft(rest[1:], " \t\f")
	}
	return line[:end], rest
}

// blank reports the whitespace of the properties format.
func blank(b byte) bool {
	return b == ' ' || b == '\t' || b == '\f'
}

// unescape decodes the escapes of the subset; a \u sequence never reaches
// it for a read key, the caller names it as Unmeasured.
func unescape(text string) string {
	if !strings.Contains(text, `\`) {
		return text
	}
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) {
			i++
			out = append(out, escaped(text[i]))
			continue
		}
		out = append(out, text[i])
	}
	return string(out)
}

// escaped is the byte an escape stands for: the four control escapes, or
// the character itself.
func escaped(b byte) byte {
	switch b {
	case 't':
		return '\t'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 'f':
		return '\f'
	}
	return b
}

// splitList splits a comma separated list, trims each entry and drops the
// empty ones; an empty value is an empty list, never nil.
func splitList(value string) []string {
	out := []string{}
	for entry := range strings.SplitSeq(value, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			out = append(out, entry)
		}
	}
	return out
}
