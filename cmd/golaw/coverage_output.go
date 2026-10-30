package main

import "github.com/cristianargotti/golaw/internal/coverage"

// coverageLines labels profiles by flag so the labels never imply a profile format.
// These are English report lines regardless of the policy language (decision 6).
// With both profiles the numbers differ on purpose: the -coverpkg profile credits
// a line that another package's tests run and the pipeline's profile does not, so
// each block names whether it is the strict floor or the number the remote gate sees.
func coverageLines(local coverage.Report, plain *coverage.Report) []string {
	if plain == nil {
		lines := append([]string{"profile --input"}, local.Lines()...)
		return append(lines, "profile --plain-input: not given")
	}
	lines := append([]string{"strict floors: profile --input"}, local.Lines()...)
	lines = append(lines, "pipeline parity: profile --plain-input")
	return append(lines, plain.Lines()...)
}
