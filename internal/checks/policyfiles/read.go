package policyfiles

import (
	"errors"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// read reads one configuration file through the snapshot of the input, so a
// check judges the worktree, the index or a revision alike. It first has the
// snapshot load every configuration path the family reads, which a snapshot
// frozen from git does in one question the first time and from memory
// after, because a question per file multiplied the git processes of a
// staged scan. A path the
// snapshot does not hold is no error: found is false and the check decides
// what an absent file means. Any other read error is returned as the
// snapshot gave it.
func read(in model.Input, path string) ([]byte, bool, error) {
	decisions := model.DefaultDecisions
	if in.Policy != nil {
		decisions = in.Policy.Layout.Decisions
	}
	model.Prefetch(in.Snapshot, configured(decisions))
	data, err := in.Snapshot.Read(path)
	if errors.Is(err, model.ErrNotInSnapshot) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err //nolint:wrapcheck // the snapshot names the path in its error
	}
	return data, true, nil
}

// answer is the result a check answers for an error of read or of a reader.
// A file over the read bound is measured with the path left unread, which
// the engine counts unmeasured outside a hook and an armed rung leaves to
// the pipeline; a stated limit keeps the text outside built, which
// model.IsOutsideSubset recognizes; any other error leaves the check
// unmeasured with its text, never clean.
func answer(path string, err error) model.Result {
	if errors.Is(err, model.ErrTooLarge) {
		return model.Result{Status: model.Measured(), Unread: []string{path}}
	}
	return model.Result{Status: model.Unmeasured(err.Error())}
}

// parseError names the file a parser refused, because the error becomes
// the Unmeasured reason a person acts on.
func parseError(path string, err error) error {
	return fmt.Errorf("parse %s: %w", path, err)
}

// outside is the error a reader answers for a construct outside its declared
// subset that a verdict depends on: the file, the line and the construct,
// then the sentinel that makes answer a stated limit.
func outside(file string, line int, construct string) error {
	return fmt.Errorf("%s:%d: %s: %w", file, line, construct, model.ErrOutsideSubset)
}

// profiled answers whether the policy enrolls a profile, and the status of a
// check the profile turns on when it does not: such a rule is profile data
// (section 5.4), and without the profile it would be a false finding.
func profiled(in model.Input) (model.Status, bool) {
	if in.Policy.Profile == "" {
		return model.NotApplicable("no profile is enrolled; the profile turns this rule on"), false
	}
	return model.Status{}, true
}

// finding builds a finding of the family at a file and a line, 0 for the
// whole file; the engine fills the check, the rule and the fingerprint.
func finding(path string, line int, code, message string) model.Finding {
	return model.Finding{Path: path, Line: line, Code: code, Message: message, Severity: model.SeverityDeny}
}
