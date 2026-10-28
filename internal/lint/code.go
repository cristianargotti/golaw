package lint

import "strings"

// Code answers the kebab form of a tool rule, the finding code: lower case,
// every run of characters outside [a-z0-9] one hyphen, trimmed at both ends.
func Code(rule string) string {
	out := make([]byte, 0, len(rule))
	pending := false
	for _, c := range []byte(strings.ToLower(rule)) {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			pending = true
			continue
		}
		if pending && len(out) > 0 {
			out = append(out, '-')
		}
		pending = false
		out = append(out, c)
	}
	return string(out)
}
