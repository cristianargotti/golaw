package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// Origin says where the policy in force came from.
type Origin int

// The three origins of a policy in force.
const (
	OriginCore      Origin = iota + 1 // no golaw.yml and no record: the core defaults
	OriginHead                        // golaw.yml at HEAD (A2)
	OriginBootstrap                   // the bootstrap policy of the record, HEAD unborn (A2)
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
	data, err := l.inForce(ctx)
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
	if err := l.enroll(); err != nil {
		return nil, err
	}
	return l, nil
}

// inForce answers the bytes of the policy in force and sets the origin:
// HEAD when it holds the file, the bootstrap record on an unborn branch,
// the core defaults for an unenrolled repository without the file.
func (l *Loaded) inForce(ctx context.Context) ([]byte, error) {
	if l.Head != "" {
		data, found, err := ReadHead(ctx, l.Root)
		if err != nil {
			return nil, err
		}
		if found {
			l.Origin = OriginHead
			return data, nil
		}
	}
	if l.Record != nil && l.Head == "" && l.Record.Bootstrap != "" {
		l.Origin = OriginBootstrap
		return []byte(l.Record.Bootstrap), nil
	}
	if l.Record != nil {
		return nil, fmt.Errorf("%w: enrolled repository %s has no %s at HEAD (%s)",
			model.ErrEnrollment, l.Root, FileName, model.EnrollmentMissing)
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
// enrolled when both digests match, ErrEnrollment otherwise.
func (l *Loaded) enroll() error {
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
		return fmt.Errorf("%w: %s of %s does not match the enrollment record (%s): run init --apply to enroll the reviewed policy",
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
	data, found, err := ReadIndex(ctx, inForce.Root)
	if err != nil || !found {
		return File{}, false, err
	}
	next, err := Decode(data)
	if err != nil {
		return File{}, false, err
	}
	if err := next.Validate(schemas); err != nil {
		return File{}, false, err
	}
	if Digest(next) == inForce.Policy.Digest {
		return File{}, false, nil
	}
	if inForce.Origin != OriginCore {
		if err := CheckTightening(inForce.File, next, schemas); err != nil {
			return File{}, false, err
		}
	}
	return next, true, nil
}
