package model

import "slices"

// The core defaults every policy starts from; a profile or a repository
// policy may only tighten them.
const (
	DefaultFileLines     = 300
	DefaultFunctionLines = 40
	DefaultLanguage      = "pt-BR"
)

// The layout a policy that names none holds: the main package at the root
// and the decision records under docs/adr (decision 5, section 5.3).
const (
	DefaultMain      = "root"
	DefaultDecisions = "docs/adr"
)

// Layout is where the main package and the decision records of the
// repository live, as golaw.yml names them.
type Layout struct {
	Main      string // root or cmd
	Decisions string // the slash separated directory of the decision records
}

// SizeFloors are the size laws of section 3.
type SizeFloors struct {
	FileLines     int // at most this many lines per Go file
	FunctionLines int // at most this many lines per function, comments ignored
}

// TermPolicy is the configurable tool-name list of text.tool-terms: each
// tool name is forbidden everywhere except under the allowed path prefixes.
type TermPolicy struct {
	Tools      []string // lower case names, sorted
	AllowPaths []string // slash separated prefixes, sorted
}

// ShellPolicy holds the optional shell denials of A7.
type ShellPolicy struct {
	DenyPush bool // deny git push
	DenyPR   bool // deny gh pr create and gh repo create
}

// EnrollmentState is what the enrollment record of A3 says about the
// repository the policy was loaded for.
type EnrollmentState int

// The enrollment states a hook or scan consults first.
const (
	EnrollmentNone     EnrollmentState = iota + 1 // no record: a positively unenrolled location
	EnrollmentEnrolled                            // the record matches the policy in force
	EnrollmentMismatch                            // the record exists and the digests differ
	EnrollmentMissing                             // the record exists and golaw.yml is absent
)

// String names the state for doctor and errors.
func (s EnrollmentState) String() string {
	switch s {
	case EnrollmentNone:
		return "unenrolled"
	case EnrollmentEnrolled:
		return "enrolled"
	case EnrollmentMismatch:
		return "mismatch"
	case EnrollmentMissing:
		return "missing-policy"
	default:
		return unknownName
	}
}

// Enrollment is the record of A3 as the policy package read it.
type Enrollment struct {
	State         EnrollmentState
	PolicyDigest  string // the digest the record expects of golaw.yml
	ProfileDigest string // the digest the record expects of the profile bundle, empty for core
}

// Exception is one committed, unexpired exception of golaw.yml: the engine
// demotes a matching finding to a note and names the record.
type Exception struct {
	Check  string // the spec id
	Path   string // the exact slash separated path
	Record string // the decision record that justifies it
}

// EffectivePolicy is the one immutable policy of a run: compiled by the
// policy package from the policy in force, the profile and the enrollment
// record, then shared by pointer and never mutated.
type EffectivePolicy struct {
	Digest       string             // sha256 of the canonical policy, printed by every report
	KitVersion   string             // the exact kit version pinned by golaw.yml (A10)
	Profile      string             // the enrolled profile name, empty for core only
	Language     string             // the human channel language, DefaultLanguage unless configured
	Capabilities []string           // the enabled capabilities, sorted, unique
	Sizes        SizeFloors         // the size laws in force
	Terms        TermPolicy         // the tool-name list and its allowed paths
	Shell        ShellPolicy        // the optional shell denials
	Enrollment   Enrollment         // what the enrollment record says
	Exceptions   []Exception        // committed, unexpired exceptions
	Options      map[string]Options // resolved options per check id
	Ladder       Ladder             // the optional local rungs this checkout arms; never a floor
	Waivers      []Waiver           // every committed vulnerability waiver, sorted by id; the push rung judges their expiry
	Layout       Layout             // where the main package and the decision records live
	ProfileData  *ProfileData       // the data of the enrolled bundle (section 5.4), nil for core only
}

// CoreDefaults is the policy every repository gets before its golaw.yml
// tightens it: core only, every size law at its default, no tool names, no
// shell denials, the default language, unenrolled.
func CoreDefaults() EffectivePolicy {
	return EffectivePolicy{
		Language: DefaultLanguage,
		Ladder:   DefaultLadder(),
		Layout:   Layout{Main: DefaultMain, Decisions: DefaultDecisions},
		Sizes:    SizeFloors{FileLines: DefaultFileLines, FunctionLines: DefaultFunctionLines},
		Enrollment: Enrollment{
			State: EnrollmentNone,
		},
	}
}

// Has reports whether a capability is enabled.
func (p *EffectivePolicy) Has(capability string) bool {
	return slices.Contains(p.Capabilities, capability)
}

// OptionsFor returns the resolved options of one check, empty when the
// policy names none; the engine merges the schema defaults underneath.
func (p *EffectivePolicy) OptionsFor(id string) Options {
	if p.Options == nil {
		return Options{}
	}
	return p.Options[id]
}

// Excepted returns the record of a committed exception covering the check
// and the path, or false.
func (p *EffectivePolicy) Excepted(check, path string) (string, bool) {
	for _, e := range p.Exceptions {
		if e.Check == check && e.Path == path {
			return e.Record, true
		}
	}
	return "", false
}
