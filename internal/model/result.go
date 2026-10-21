package model

import "strings"

// StatusKind says whether a check measured anything.
type StatusKind int

// The three kinds of section 5.2. The zero value is invalid, so a check
// that forgets its status is reported as unmeasured by the engine.
const (
	StatusMeasured      StatusKind = iota + 1 // the check ran over its scope
	StatusNotApplicable                       // the check does not apply here, with a reason
	StatusUnmeasured                          // the check could not run, with a reason; never clean
)

// String names the kind for reports.
func (k StatusKind) String() string {
	switch k {
	case StatusMeasured:
		return "measured"
	case StatusNotApplicable:
		return "not-applicable"
	case StatusUnmeasured:
		return "unmeasured"
	default:
		return unknownName
	}
}

// Valid reports whether the kind is one of the three.
func (k StatusKind) Valid() bool { return k >= StatusMeasured && k <= StatusUnmeasured }

// Status is the kind with its reason; a measured status has none.
type Status struct {
	Kind   StatusKind
	Reason string
}

// Measured is the status of a check that ran.
func Measured() Status { return Status{Kind: StatusMeasured} }

// NotApplicable is the status of a check that does not apply, with why.
func NotApplicable(reason string) Status {
	return Status{Kind: StatusNotApplicable, Reason: reason}
}

// Unmeasured is the status of a check that could not run, with why.
func Unmeasured(reason string) Status {
	return Status{Kind: StatusUnmeasured, Reason: reason}
}

// OutsideSubset is the status of a policy check whose verdict depends on a
// construct outside the subset its reader declares: Unmeasured, so the
// pipeline never answers clean over it (section 5.4), with what names the
// file, the line and the construct, then the sentinel an armed rung reads to
// leave the construct to the pipeline instead of refusing the commit.
func OutsideSubset(what string) Status {
	return Unmeasured(what + ": " + ErrOutsideSubset.Error())
}

// IsOutsideSubset reports whether a status is the stated limit of a declared
// subset, as OutsideSubset builds it.
func IsOutsideSubset(s Status) bool {
	return s.Kind == StatusUnmeasured && strings.HasSuffix(s.Reason, ": "+ErrOutsideSubset.Error())
}

// Result is what a check returns: its status, what it measured and what it
// found. A required check left Unmeasured fails the run. Unread names the
// files a measured check left unread because they are over the read
// bound, a target or a file it reads beside one, having judged everything
// else: a hook names them in a notice, every other mode counts the check
// unmeasured.
type Result struct {
	Status   Status
	Scope    string    // what was measured, for the report: "412 files", "1 command"
	Findings []Finding // may be empty; the engine sorts and fingerprints them
	Unread   []string  // the files over the read bound the check could not read
}
