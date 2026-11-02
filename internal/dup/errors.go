package dup

import "errors"

var (
	// ErrOptions marks bounds that would count every token as a copy.
	ErrOptions = errors.New("duplication bounds must be positive")
	// ErrTokenize marks a Go file the scanner rejects, so its lines are
	// never silently counted as unique.
	ErrTokenize = errors.New("go source does not tokenize")
)
