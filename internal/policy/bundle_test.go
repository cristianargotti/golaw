package policy_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// acmeData is the data a loader answers for the example bundle.
func acmeData() *model.ProfileData {
	return &model.ProfileData{
		Name: "acme", Version: "0.3.0", Digest: "sha256:" + acmeDigest,
		Terms: []string{"zeta", "acme-tool"}, Paths: []string{"NOTES.md"},
	}
}

func TestCompileWithMergesTheProfileData(t *testing.T) {
	t.Parallel()
	f := decode(t, fullPolicy)
	p, err := policy.CompileWith(f, now(), schemas(), acmeData())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Terms.Tools, []string{"acme-tool", "other", "zeta"}) {
		t.Errorf("the profile terms join the tool list sorted and unique: %v", p.Terms.Tools)
	}
	if p.ProfileData == nil || p.ProfileData.Name != "acme" || !p.ProfileData.Section(model.SectionPaths) {
		t.Errorf("the data is carried: %+v", p.ProfileData)
	}
	if !slices.Equal(p.Terms.AllowPaths, []string{"agents/", "docs/"}) {
		t.Errorf("the allow paths are the file's: %v", p.Terms.AllowPaths)
	}
	plain, err := policy.CompileWith(f, now(), schemas(), nil)
	if err != nil || plain.ProfileData != nil || !slices.Equal(plain.Terms.Tools, []string{"acme-tool", "other"}) {
		t.Errorf("a nil loader answer compiles as Compile does: %+v, %v", plain, err)
	}
	other := acmeData()
	other.Name = "other"
	if _, err := policy.CompileWith(f, now(), schemas(), other); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("data of another profile is refused: %v", err)
	}
	core := decode(t, minimalPolicy)
	if _, err := policy.CompileWith(core, now(), schemas(), acmeData()); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("data under a core-only file is refused: %v", err)
	}
}

func TestLoadReadsTheBundleThroughTheLoader(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	var seen policy.ProfileRef
	in := input(root, d)
	in.Bundle = func(configDir string, ref policy.ProfileRef) (*model.ProfileData, error) {
		if configDir != d.Config {
			t.Errorf("the loader reads the config directory of the dirs: %s", configDir)
		}
		seen = ref
		return acmeData(), nil
	}
	l, err := policy.Load(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	if seen != acmeRef() {
		t.Errorf("the loader is handed the pinned reference: %+v", seen)
	}
	if l.Data == nil || l.Policy.ProfileData == nil || l.Policy.ProfileData.Name != "acme" {
		t.Errorf("the data lands on the loaded policy: %+v", l.Policy.ProfileData)
	}
	if !slices.Equal(l.Policy.Terms.Tools, []string{"acme-tool", "other", "zeta"}) {
		t.Errorf("the terms in force hold the profile's: %v", l.Policy.Terms.Tools)
	}
	if l.Stamp == nil || l.Policy.Enrollment.State != model.EnrollmentEnrolled {
		t.Errorf("the stamp and the enrollment stay as M3 left them: %+v, %+v", l.Stamp, l.Policy.Enrollment)
	}
}

func TestLoadWithoutALoaderVerifiesTheStampAlone(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	l, err := policy.Load(t.Context(), input(root, d))
	if err != nil {
		t.Fatal(err)
	}
	if l.Data != nil || l.Policy.ProfileData != nil || l.Stamp == nil {
		t.Errorf("a nil loader keeps the M3 behavior: data %+v, stamp %+v", l.Data, l.Stamp)
	}
}

func TestLoadReturnsTheLoaderErrorAsGiven(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, fullPolicy)
	in := input(root, d)
	in.Bundle = func(string, policy.ProfileRef) (*model.ProfileData, error) {
		return nil, fmt.Errorf("%w: profile acme: digest sha256:0 of the installed bundle differs from its stamp", model.ErrPolicy)
	}
	_, err := policy.Load(t.Context(), in)
	if !errors.Is(err, model.ErrPolicy) || err.Error() != "invalid policy: profile acme: digest sha256:0 of the installed bundle differs from its stamp" {
		t.Errorf("the loader error is the load error: %v", err)
	}
}

func TestLoadNeverCallsTheLoaderWithoutAProfile(t *testing.T) {
	t.Parallel()
	root, d := enrolled(t, minimalPolicy)
	in := input(root, d)
	in.Bundle = func(string, policy.ProfileRef) (*model.ProfileData, error) {
		t.Error("a core-only policy reads no bundle")
		return nil, nil //nolint:nilnil // the fake never answers
	}
	if l, err := policy.Load(t.Context(), in); err != nil || l.Data != nil || l.Policy.ProfileData != nil {
		t.Errorf("Load = %+v, %v", l, err)
	}
}
