package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
)

// Severity is what the check thinks of a finding; the engine decides the
// enforcement, so a severity is descriptive, never a verdict.
type Severity int

// The three severities of section 5.2.
const (
	SeverityDeny Severity = iota + 1 // a hook denies on it, a scan fails on it
	SeverityFail                     // a scan fails on it, a hook denies on it
	SeverityNote                     // reported, never enforced
)

// Valid reports whether the severity is one of the three.
func (s Severity) Valid() bool { return s >= SeverityDeny && s <= SeverityNote }

// String names the severity; it is also the JSON form.
func (s Severity) String() string {
	switch s {
	case SeverityDeny:
		return "deny"
	case SeverityFail:
		return "fail"
	case SeverityNote:
		return "note"
	default:
		return unknownName
	}
}

// MarshalText writes the severity name for the JSON report.
func (s Severity) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// UnmarshalText reads a severity name; an unknown one is a usage error.
func (s *Severity) UnmarshalText(text []byte) error {
	for candidate := SeverityDeny; candidate <= SeverityNote; candidate++ {
		if candidate.String() == string(text) {
			*s = candidate
			return nil
		}
	}
	return fmt.Errorf("%w: unknown severity %q", ErrUsage, string(text))
}

// Finding is one violation. Check and Rule are filled by the engine from
// the spec when a check leaves them empty, and so is Fingerprint.
type Finding struct {
	Check       string   `json:"check"`            // the spec id
	Rule        string   `json:"rule"`             // the spec rule reference
	Path        string   `json:"path,omitempty"`   // slash separated, empty for an event finding
	Line        int      `json:"line,omitempty"`   // 1-based, 0 when the finding has no line
	Column      int      `json:"column,omitempty"` // 1-based, 0 when unknown
	Code        string   `json:"code"`             // the stable code the proof manifest expects, kebab-case
	Message     string   `json:"message"`          // English, one sentence, never a line number inside it
	Fingerprint string   `json:"fingerprint"`      // rule, check, path, code and normalized message; no line number
	Severity    Severity `json:"severity"`         // descriptive; the engine decides enforcement
	Sonar       string   `json:"sonar,omitempty"`  // the Sonar key a mapped finding answers to, javascript:S2681; never in the fingerprint
}

// Enforced reports whether the finding counts against the run.
func (f Finding) Enforced() bool { return f.Severity != SeverityNote }

// Fingerprint identifies a finding across line shifts: the rule version,
// the check, the path, the code and the message with its whitespace
// collapsed, hashed. Staged mode compares fingerprints as a multiset.
func Fingerprint(f Finding) string {
	message := strings.Join(strings.Fields(f.Message), " ")
	sum := sha256.Sum256([]byte(strings.Join([]string{f.Rule, f.Check, f.Path, f.Code, message}, "\x00")))
	return hex.EncodeToString(sum[:])
}

// SortFindings orders findings deterministically: path, line, column,
// check, code, message, fingerprint.
func SortFindings(findings []Finding) {
	slices.SortStableFunc(findings, compareFindings)
}

func compareFindings(a, b Finding) int {
	if c := strings.Compare(a.Path, b.Path); c != 0 {
		return c
	}
	if a.Line != b.Line {
		return a.Line - b.Line
	}
	if a.Column != b.Column {
		return a.Column - b.Column
	}
	for _, pair := range [][2]string{{a.Check, b.Check}, {a.Code, b.Code}, {a.Message, b.Message}} {
		if c := strings.Compare(pair[0], pair[1]); c != 0 {
			return c
		}
	}
	return strings.Compare(a.Fingerprint, b.Fingerprint)
}
