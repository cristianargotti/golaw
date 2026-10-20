package policy

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// LoadNext is Load for what judges the next commit, the git rungs. The
// enrolled golaw.yml the index holds is in force while HEAD holds none
// (Uncommitted, as the agent hooks load), and so is the golaw.yml init
// --update wrote while HEAD still pins the version before it: once it is
// staged, pins this binary and is the policy the record enrolled, the
// commit that records the upgrade passes the rungs of the binary that made
// it (A10, section 9). While the index lacks it the refusal asks for that
// commit; every other refusal of Load stands.
func LoadNext(ctx context.Context, in Input) (*Loaded, error) {
	in.Uncommitted = true
	l, err := Load(ctx, in)
	if err == nil {
		return l, nil
	}
	loc, ok := upgrading(ctx, in)
	if !ok {
		return nil, err
	}
	return upgraded(ctx, in, loc)
}

// upgrading reports whether a refusal of Load is the pin of an upgrade the
// record already enrolled: this binary is a release, the record enrolled a
// policy for its version, and HEAD holds a golaw.yml that pins another one.
func upgrading(ctx context.Context, in Input) (Location, bool) {
	if in.Version == "" {
		return Location{}, false
	}
	loc, err := Locate(ctx, in.Dirs.State, in.Dir, in.Discover)
	if err != nil || loc.Record == nil || loc.Head == "" || loc.Record.KitVersion != in.Version {
		return Location{}, false
	}
	head := (&Loaded{Root: loc.Root}).reads().atHead(ctx)
	if head.err != nil || !head.found {
		return Location{}, false
	}
	f, err := Decode(head.data)
	return loc, err == nil && f.Kit.Version != in.Version
}

// upgraded is the policy in force for the commit of an upgrade: the staged
// golaw.yml that pins this binary and is the policy the record enrolled,
// compiled and enrolled as Load does.
func upgraded(ctx context.Context, in Input, loc Location) (*Loaded, error) {
	l := &Loaded{Root: loc.Root, Head: loc.Head, Record: loc.Record, Schemas: in.Schemas, Origin: OriginStaged}
	staged := l.reads().staged(ctx)
	if staged.err != nil {
		return nil, staged.err
	}
	f, err := Decode(staged.data)
	if !staged.found || err != nil || f.Kit.Version != in.Version || Digest(f) != loc.Record.PolicyDigest {
		return nil, fmt.Errorf("%w: %s at HEAD of %s pins another kit version and init --update enrolled the one of this binary, %s: stage and commit the %s init --update wrote (git add %s)",
			model.ErrPolicy, FileName, l.Root, in.Version, FileName, FileName)
	}
	l.File = f
	if l.Policy, err = Compile(f, in.Now, in.Schemas); err != nil {
		return nil, err
	}
	if err := l.verifyProfile(in); err != nil {
		return nil, err
	}
	if err := l.enroll(false, in.Schemas); err != nil {
		return nil, err
	}
	return l, nil
}
