//go:build ruleguard

// Package gorules holds a rules file whose only rule is not the byte cut
// rule of the standard.
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// sprintString: a string formatted with %s alone is the string itself.
func sprintString(m dsl.Matcher) {
	m.Match(`fmt.Sprintf("%s", $s)`).
		Where(m["s"].Type.Is(`string`)).
		Report(`use the string itself`)
}
