//go:build ruleguard

// Package gorules holds the ruleguard rules golangci-lint runs through
// gocritic: the type-aware checks the standard needs and no shipped linter
// carries on its own. The build tag keeps the file out of every build, and
// its import keeps the dsl module in go.mod.
package gorules

import "github.com/quasilyte/go-ruleguard/dsl"

// stringByteCut: a string cut by a byte count can split a multibyte rune
// and leave text that is not valid UTF-8; cut at a rune start instead. A
// cut of text that is ASCII by construction (hex, digits, a base32 token)
// says so in a nolint reason.
func stringByteCut(m dsl.Matcher) {
	m.Match(`if len($s) > $n { $s = $s[:$n] }`).
		Where(m["s"].Type.Is(`string`)).
		Report(`a byte cut of a string can split a rune: cut at a rune start`)
	m.Match(`$s[:$n]`).
		Where(m["s"].Type.Is(`string`) && m["n"].Const).
		Report(`a byte cut of a string can split a rune: cut at a rune start`)
}
