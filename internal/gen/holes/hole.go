package holes

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
)

// Marker opens the one line comment that marks a hole (M7.4.2 rule 1); only
// a line comment counts, so a quoted or block form never opens one.
const Marker = "golaw:hole"

// SupportName is the import name the hole call is recognized by: a Hole
// function of any other package is the adopter's own and never a hole.
const SupportName = "gensupport"

// Hole is one open hole of an owned file, derived from the source.
type Hole struct {
	ID         string // "<name>.<method in kebab case>"
	Constraint string // the rest of the marker line
	Path       string
	Line       int    // the line of the marker, or of the call without one
	Func       string // "(*Plans).ReadPlan", "ReadPlan" for a function, "Ledger" for an interface stub
}

// Kebab spells a Go name in kebab case, keeping an initialism together so
// "GetHTTPStatus" is "get-http-status" and a hole id reads as words.
func Kebab(name string) string {
	runes := []rune(name)
	out := make([]rune, 0, 2*len(runes))
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && wordStart(runes, i) {
			out = append(out, '-')
		}
		out = append(out, unicode.ToLower(r))
	}
	return string(out)
}

// wordStart reports whether the upper case rune at i begins a new word:
// after a lower case rune or a digit, or as the last capital of an
// initialism that a lower case rune follows.
func wordStart(runes []rune, i int) bool {
	prev := runes[i-1]
	if unicode.IsLower(prev) || unicode.IsDigit(prev) {
		return true
	}
	return unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1])
}

// ID is the hole id of one method of a named contract.
func ID(name, method string) string {
	return strings.ToLower(name) + "." + Kebab(method)
}

// validID admits lower case words joined by dots and hyphens, so a marker
// that only resembles a hole id never counts as one.
func validID(id string) bool {
	if !strings.Contains(id, ".") {
		return false
	}
	for part := range strings.SplitSeq(id, ".") {
		if part == "" || part[0] == '-' || part[len(part)-1] == '-' {
			return false
		}
		for _, c := range []byte(part) {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return true
}

// Diff compares the holes of two runs by id alone, because a hole moves
// lines whenever code above it changes and is still the same hole. It
// answers the opened holes, those of after, then the closed ones, those of
// before.
func Diff(before, after []Hole) ([]Hole, []Hole) {
	return missing(after, before), missing(before, after)
}

// missing answers the holes of from whose id other lacks, in path and line
// order, nil when there is none.
func missing(from, other []Hole) []Hole {
	ids := make(map[string]bool, len(other))
	for _, h := range other {
		ids[h.ID] = true
	}
	var out []Hole
	for _, h := range from {
		if !ids[h.ID] {
			out = append(out, h)
		}
	}
	slices.SortFunc(out, func(a, b Hole) int {
		return cmp.Or(cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	return out
}
