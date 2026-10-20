package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// Origin says where the policy in force came from.
type Origin int

// The origins of a policy in force.
const (
	OriginCore      Origin = iota + 1 // no golaw.yml and no record: the core defaults
	OriginHead                        // golaw.yml at HEAD (A2)
	OriginBootstrap                   // the bootstrap policy of the record, HEAD unborn (A2)
	OriginStaged                      // the enrolled golaw.yml the index holds, not at HEAD yet (hook only)
	OriginCheckout                    // the enrolled golaw.yml the checkout holds, not staged yet (hook only)
)

// String names the origin for config explain.
func (o Origin) String() string {
	switch o {
	case OriginCore:
		return "core"
	case OriginHead:
		return "HEAD:" + FileName
	case OriginBootstrap:
		return "bootstrap record"
	case OriginStaged:
		return "index:" + FileName
	case OriginCheckout:
		return "checkout:" + FileName
	default:
		return "unknown"
	}
}

// Input is what Load needs: where the run stands, the directories outside
// the checkout, the running binary's version (empty for a development
// build, which skips the pin), the clock, the option schemas of the
// catalog and the discoverer.
type Input struct {
	Dir      string
	Dirs     Dirs
	Version  string
	Now      time.Time
	Schemas  Schemas
	Discover Discoverer
	// Reenroll is set by a record writer (A3): a record whose digests
	// differ from the policy in force is answered with the mismatch state
	// instead of an error, once the policy in force tightens the policy
	// the record enrolled (CheckReenrollment).
	Reenroll bool
	// Uncommitted is set by the hook: while HEAD holds no golaw.yml yet,
	// the policy in force is the enrolled one the index or the checkout
	// holds, when its bytes mean the policy the record enrolled, so an
	// agent works in a repository init --apply just enrolled.
	Uncommitted bool
}

// Loaded is the policy in force with everything config explain reports.
type Loaded struct {
	Policy  *model.EffectivePolicy
	File    File // zero when the origin is core
	Root    string
	Head    string
	Origin  Origin
	Record  *Record
	Stamp   *Stamp
	Schemas Schemas // the option schemas of the catalog, whose defaults explain lists
	git     *reads  // the answers git gave about golaw.yml, each asked once
}

// Load establishes the policy in force for a directory: the enrollment
// and the repository (Locate), the file at HEAD or the bootstrap policy
// of the record, strict decoding, the kit pin (A10), the profile stamp,
// compilation and the enrollment state. An enrolled repository whose
// policy is missing or whose digests differ is ErrEnrollment; a file that
// breaks the schema or the floors is ErrPolicy; an unenrolled repository
// without golaw.yml gets the core defaults.
func Load(ctx context.Context, in Input) (*Loaded, error) {
	loc, err := Locate(ctx, in.Dirs.State, in.Dir, in.Discover)
	if err != nil {
		return nil, err
	}
	l := &Loaded{Root: loc.Root, Head: loc.Head, Record: loc.Record, Schemas: in.Schemas}
	data, err := l.inForce(ctx, in.Uncommitted)
	if err != nil {
		return nil, err
	}
	if l.Origin == OriginCore {
		p := model.CoreDefaults()
		l.Policy = &p
		return l, nil
	}
	if l.File, err = Decode(data); err != nil {
		return nil, err
	}
	if in.Version != "" && l.File.Kit.Version != in.Version {
		return nil, fmt.Errorf("%w: %s pins kit version %s and this binary is %s: install the pinned version or run init --update",
			model.ErrPolicy, FileName, l.File.Kit.Version, in.Version)
	}
	if l.Policy, err = Compile(l.File, in.Now, in.Schemas); err != nil {
		return nil, err
	}
	if err := l.verifyProfile(in.Dirs.Config); err != nil {
		return nil, err
	}
	if err := l.enroll(in.Reenroll, in.Schemas); err != nil {
		return nil, err
	}
	return l, nil
}

// inForce answers the bytes of the policy in force and sets the origin:
// HEAD when it holds the file, the bootstrap record on an unborn branch,
// for the hook the enrolled policy of the index or the checkout while
// HEAD holds none, the core defaults for an unenrolled repository without
// the file.
func (l *Loaded) inForce(ctx context.Context, uncommitted bool) ([]byte, error) {
	if l.Head != "" {
		head := l.reads().atHead(ctx)
		if head.err != nil {
			return nil, head.err
		}
		if head.found {
			l.Origin = OriginHead
			return head.data, nil
		}
	}
	if l.Record != nil && l.Head == "" && l.Record.Bootstrap != "" {
		l.Origin = OriginBootstrap
		return []byte(l.Record.Bootstrap), nil
	}
	if l.Record != nil && uncommitted {
		return l.uncommitted(ctx)
	}
	if l.Record != nil {
		return nil, fmt.Errorf("%w: enrolled repository %s has no %s at HEAD (%s): %w",
			model.ErrEnrollment, l.Root, FileName, model.EnrollmentMissing, ErrNotAtHead)
	}
	l.Origin = OriginCore
	return nil, nil
}

// verifyProfile checks the installed bundle when the file names one.
func (l *Loaded) verifyProfile(configDir string) error {
	if l.File.Profile == nil {
		return nil
	}
	s, err := VerifyProfile(configDir, *l.File.Profile)
	if err != nil {
		return err
	}
	l.Stamp = &s
	return nil
}

// enroll fills the enrollment state from the record: none without one,
// enrolled when both digests match, ErrEnrollment otherwise, unless a
// record writer re-enrolls a tightening.
func (l *Loaded) enroll(reenroll bool, schemas Schemas) error {
	if l.Record == nil {
		l.Policy.Enrollment = model.Enrollment{State: model.EnrollmentNone}
		return nil
	}
	profile := ""
	if l.File.Profile != nil {
		profile = l.File.Profile.Digest
	}
	l.Policy.Enrollment = model.Enrollment{
		State:         model.EnrollmentEnrolled,
		PolicyDigest:  l.Record.PolicyDigest,
		ProfileDigest: l.Record.ProfileDigest,
	}
	if l.Record.PolicyDigest != l.Policy.Digest || l.Record.ProfileDigest != profile {
		l.Policy.Enrollment.State = model.EnrollmentMismatch
		if reenroll {
			return CheckReenrollment(*l.Record, l.File, schemas)
		}
		return fmt.Errorf("%w: %s of %s does not match the enrollment record (%s): review the change, then run agent install to enroll the policy in force",
			model.ErrEnrollment, FileName, l.Root, model.EnrollmentMismatch)
	}
	return nil
}

// Proposed reads the next policy the index holds and checks it as a
// tightening of the one in force (A2). It answers false when nothing is
// staged at golaw.yml or when the staged file means the policy already in
// force; without a policy in force only validation applies. The staged
// file is validated before its digest is compared, so a file that hashes
// like the policy in force is still refused when it breaks the schema.
func Proposed(ctx context.Context, inForce *Loaded, schemas Schemas) (File, bool, error) {
	proposal, err := Propose(ctx, inForce, schemas)
	return proposal.File, proposal.Found, err
}

// Proposal is what the index holds against the policy in force: the next
// policy when it differs, and whether the index holds golaw.yml at all.
type Proposal struct {
	File   File
	Found  bool
	Staged bool
}

// Propose is Proposed that also answers whether the index holds golaw.yml,
// from one listing of the index, so a caller that refuses a staged deletion
// asks git nothing more; a staged blob that is the one at HEAD is not read
// again. A policy in force the load took from the index or the checkout
// before the first commit answers from what that load read: the staged
// policy is the one in force, and a checkout policy means nothing staged.
func Propose(ctx context.Context, inForce *Loaded, schemas Schemas) (Proposal, error) {
	switch inForce.Origin {
	case OriginStaged:
		return Proposal{Staged: true}, nil
	case OriginCheckout:
		return Proposal{}, nil
	case OriginHead, OriginBootstrap, OriginCore:
	}
	staged := inForce.reads().staged(ctx)
	if staged.err != nil || !staged.found {
		return Proposal{}, staged.err
	}
	return inForce.judge(staged.data, schemas)
}

// reads is the memo of the git answers about golaw.yml under the root,
// made on first use for a Loaded built without Load.
func (l *Loaded) reads() *reads {
	if l.git == nil || l.git.root != l.Root {
		l.git = &reads{root: l.Root}
	}
	return l.git
}
