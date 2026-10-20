package main

import "github.com/cristianargotti/golaw/internal/coverage"

// coverageLines labels profiles by flag so the labels never imply a profile format.
// These are English report lines regardless of the policy language (decision 6).
func coverageLines(local coverage.Report, plain *coverage.Report) []string {
	lines := append([]string{"profile --input"}, local.Lines()...)
	if plain == nil {
		return append(lines, "profile --plain-input: not given")
	}
	lines = append(lines, "profile --plain-input")
	return append(lines, plain.Lines()...)
}
