package match

import "regexp"

// Valid reports an identifier.
func Valid(s string) bool {
	return regexp.MustCompile(`^[a-z]+$`).MatchString(s)
}
