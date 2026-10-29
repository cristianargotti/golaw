// Package surface holds the native checks of the non-Go surface of a Go
// repository (M6.7): recognizers over the tokens of a small lexer per
// language, never a linter, so each one judges only what its predicate
// reads with certainty and leaves the rest to the pinned linters of the
// lint target. The family reads the model and the detection of
// internal/surface, imported as detect.
package surface
