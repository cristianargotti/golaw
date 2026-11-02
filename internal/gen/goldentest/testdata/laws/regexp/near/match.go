package match

import "strings"

// Valid reports an identifier; regexp is never imported here.
func Valid(s string) bool {
	return s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyz") == ""
}
