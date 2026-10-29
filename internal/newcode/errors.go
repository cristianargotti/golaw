// Package newcode measures coverage on new code the way the pipeline's
// gate does (M6.8): Sonar's Go line model over the pipeline report, the
// lines a change adds since its base, the analyzer scope's universe and
// the Sonar way threshold.
package newcode

import "errors"

var (
	// ErrReport marks a report golaw cannot measure: missing, empty,
	// unparseable, naming no file of the universe, or older than the source.
	ErrReport = errors.New("coverage report unusable")
	// ErrModule marks a go.mod without a module directive.
	ErrModule = errors.New("go.mod names no module")
	// ErrBase marks a base the repository cannot resolve: an unknown ref,
	// a history unrelated to HEAD (a shallow clone has none), no tag.
	ErrBase = errors.New("new code base unresolvable")
	// ErrDiff marks a git answer golaw cannot read as the new lines.
	ErrDiff = errors.New("git could not name the new lines")
	// ErrUnmeasured marks a measure the command could not take; it exits 2
	// on it, never 0, so a missing number never reads as a pass (M6.8 item 6).
	ErrUnmeasured = errors.New("coverage on new code unmeasured")
)
