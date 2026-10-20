package model

import (
	"context"
	"fmt"
	"regexp"
	"slices"
)

// Scope says over what a check measures. The engine reads it to decide the
// modes and event shapes in which the check runs; the zero value is invalid
// so that a forgotten scope is refused at registry build instead of silently
// becoming a file check.
type Scope int

// The four scopes of section 5.2 of the specification.
const (
	ScopeFile   Scope = iota + 1 // one measurement over the target files
	ScopeTree                    // one measurement over the whole snapshot
	ScopeEvent                   // one measurement over a hook event
	ScopeCommit                  // one measurement over commit message files
)

// String names the scope for reports and errors.
func (s Scope) String() string {
	switch s {
	case ScopeFile:
		return "file"
	case ScopeTree:
		return "tree"
	case ScopeEvent:
		return "event"
	case ScopeCommit:
		return "commit"
	default:
		return unknownName
	}
}

// Valid reports whether the scope is one of the four.
func (s Scope) Valid() bool { return s >= ScopeFile && s <= ScopeCommit }

// Mode is the way the kit was invoked; applicability and the decision
// depend on it.
type Mode int

// The four modes of section 5.2: a worktree scan, a staged scan over the
// index, an agent hook and a commit message rung.
const (
	ModeScan Mode = iota + 1
	ModeStaged
	ModeHook
	ModeCommit
)

// String names the mode for reports and errors.
func (m Mode) String() string {
	switch m {
	case ModeScan:
		return "scan"
	case ModeStaged:
		return "staged"
	case ModeHook:
		return "hook"
	case ModeCommit:
		return "commit"
	default:
		return unknownName
	}
}

// Valid reports whether the mode is one of the four.
func (m Mode) Valid() bool { return m >= ModeScan && m <= ModeCommit }

// OwnerCore is the owner of every check of the core ruleset; any other owner
// is a profile name, and the check applies only under that profile.
const OwnerCore = "core"

// Capabilities lists every capability a policy may enable and a Spec may
// need, sorted. It is a function because the lint configuration refuses
// mutable package variables.
func Capabilities() []string {
	return []string{"api", "cli", "db", "go", "hexagonal", "image"}
}

// Requirements lists every runtime need a Spec may declare, sorted.
func Requirements() []string {
	return []string{"external-evidence", "git-history", "go-types"}
}

var (
	idPattern    = regexp.MustCompile(`^[a-z]+\.[a-z][a-z0-9-]*$`)
	rulePattern  = regexp.MustCompile(`^rules/\d+(\.\d+)?@\d+$`)
	ownerPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// ValidID reports whether an id has the family.name grammar of a check id
// and of a proof key.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// ValidCode reports whether a finding code is lower case kebab-case.
func ValidCode(code string) bool { return ownerPattern.MatchString(code) }

// Spec describes one check: what it is, where it applies and which proof
// pair backs it. Every field is validated at registry build.
type Spec struct {
	ID       string       // "text.dashes": family, dot, name; unique
	Rule     string       // stable rule reference plus version: "rules/3@1"
	Scope    Scope        // what the check measures
	Owner    string       // OwnerCore or a profile name
	Needs    []string     // capabilities that must all be enabled
	Requires []string     // runtime needs the mode must provide
	Options  OptionSchema // the options the check reads from the policy
	Proof    string       // key into the proof catalog; the catalog is the authority
}

// Validate refuses a spec the registry must not accept, naming the field so
// a builder fixes the registration rather than the engine.
func (s Spec) Validate() error {
	switch {
	case !idPattern.MatchString(s.ID):
		return fmt.Errorf("%w: id %q must be family.name in lower case", ErrInvalidSpec, s.ID)
	case !rulePattern.MatchString(s.Rule):
		return fmt.Errorf("%w: %s: rule %q must look like rules/3@1", ErrInvalidSpec, s.ID, s.Rule)
	case !s.Scope.Valid():
		return fmt.Errorf("%w: %s: scope %d is not file, tree, event or commit", ErrInvalidSpec, s.ID, int(s.Scope))
	case !ownerPattern.MatchString(s.Owner):
		return fmt.Errorf("%w: %s: owner %q must be core or a profile name", ErrInvalidSpec, s.ID, s.Owner)
	case !idPattern.MatchString(s.Proof):
		return fmt.Errorf("%w: %s: proof %q must be a catalog key in family.name form", ErrInvalidSpec, s.ID, s.Proof)
	}
	for _, err := range []error{
		knownSet("needs", s.Needs, Capabilities()),
		knownSet("requires", s.Requires, Requirements()),
		s.Options.Validate(),
	} {
		if err != nil {
			return fmt.Errorf("%s: %w", s.ID, err)
		}
	}
	return nil
}

// knownSet refuses an unknown or repeated entry of a declared list.
func knownSet(field string, got, known []string) error {
	for i, name := range got {
		if !slices.Contains(known, name) {
			return fmt.Errorf("%w: %s names unknown %q", ErrInvalidSpec, field, name)
		}
		if slices.Contains(got[:i], name) {
			return fmt.Errorf("%w: %s repeats %q", ErrInvalidSpec, field, name)
		}
	}
	return nil
}

// Check is one measurable rule. Spec is pure data; Run reads bytes only
// through in.Snapshot, so the same code judges a file on disk, an index blob
// or a candidate that does not exist yet.
type Check interface {
	Spec() Spec
	Run(ctx context.Context, in Input) (Result, error)
}

// Input is what the engine hands to every applicable check. The caller fills
// Mode, Policy, Snapshot, Changes, Targets, Event and Commit; the engine
// fills Options per check. A check never mutates it.
type Input struct {
	Mode     Mode
	Policy   *EffectivePolicy // immutable, carries its digest; never nil
	Snapshot Snapshot         // frozen universe; a hook overlay is read through it; never nil
	Changes  []Change         // the staged changes in staged and commit mode, empty otherwise
	Targets  []File           // the files a file or commit scope check measures
	Event    *Event           // the normalized event in hook mode, nil otherwise
	Commit   *Commit          // the commit in commit mode, nil otherwise
	Options  Options          // the check's resolved options
}
