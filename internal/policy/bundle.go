package policy

import (
	"fmt"
	"slices"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// BundleLoader reads the data of the bundle golaw.yml names under the
// config directory, after VerifyProfile accepted its stamp (section 5.4,
// A19); cmd binds the profile package's loader, the policy tests leave it
// nil, which verifies the stamp alone as M3 did.
type BundleLoader func(configDir string, ref ProfileRef) (*model.ProfileData, error)

// CompileWith is Compile with the data of the enrolled bundle: the terms of
// the data join the tool list sorted and unique (a profile only adds, A1),
// the data rides on the policy, and the data must belong to the profile
// the file names.
func CompileWith(f File, now time.Time, schemas Schemas, data *model.ProfileData) (*model.EffectivePolicy, error) {
	p, err := Compile(f, now, schemas)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return p, nil
	}
	if p.Profile == "" || data.Name != p.Profile {
		return nil, fmt.Errorf("%w: %s names profile %q and the bundle loaded is %q", model.ErrPolicy, FileName, p.Profile, data.Name)
	}
	p.Terms.Tools = sortedUnique(slices.Concat(p.Terms.Tools, data.Terms))
	p.ProfileData = data
	return p, nil
}

// loadBundle reads the data of the pinned bundle through the loader, when
// one is bound and the file names a profile, and compiles the policy again
// with it; a loader error is the load error as given.
func (l *Loaded) loadBundle(in Input) error {
	if in.Bundle == nil || l.File.Profile == nil {
		return nil
	}
	data, err := in.Bundle(in.Dirs.Config, *l.File.Profile)
	if err != nil {
		return err
	}
	l.Data = data
	l.Policy, err = CompileWith(l.File, in.Now, in.Schemas, data)
	return err
}
