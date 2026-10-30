package model

// Ladder says which optional local rungs a checkout arms (decision 10). It
// is local convenience and never a floor: the policy digest leaves it out.
type Ladder struct {
	PreCommit  bool // golaw hook pre-commit runs
	CommitMsg  bool // golaw hook commit-msg runs
	PrePush    bool // golaw hook pre-push runs
	TestStaged bool // the pre-commit rung also runs go test over the affected packages
	// SurfaceLint has the pre-commit rung also run the fast surface linters
	// over the staged files of their languages.
	SurfaceLint bool
	// HookFeedback has the agent hooks run the fast tools at post events and
	// feed back the native checks that are not certain.
	HookFeedback bool
}

// DefaultLadder is the ladder of a policy that names none: every rung runs
// once the person armed it, test_staged stays off (decision 8), and the
// surface aids of M6 are on wherever their rung or guard is installed.
func DefaultLadder() Ladder {
	return Ladder{PreCommit: true, CommitMsg: true, PrePush: true, SurfaceLint: true, HookFeedback: true}
}

// Waiver is one dated vulnerability waiver of A6, as golaw.yml commits it.
type Waiver struct {
	ID     string // the advisory id: GO-2026-6603
	Until  string // the last day it holds, YYYY-MM-DD, in UTC
	Record string // the decision record that justifies it
}
