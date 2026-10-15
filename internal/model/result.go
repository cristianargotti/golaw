package model

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

// Result is what a check returns: its status, what it measured and what it
// found. A required check left Unmeasured fails the run.
type Result struct {
	Status   Status
	Scope    string    // what was measured, for the report: "412 files", "1 command"
	Findings []Finding // may be empty; the engine sorts and fingerprints them
}
