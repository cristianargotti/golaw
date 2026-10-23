package proof_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profiletest"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestPolicyOfCarriesTheDataOfTheNamedProfile pins D6 for the proof: the
// fixtures of an entry that names a profile run under PolicyFor with the
// data the manifest holds for that profile, nil when it holds none, and a
// core entry never carries data.
func TestPolicyOfCarriesTheDataOfTheNamedProfile(t *testing.T) {
	t.Parallel()
	data := profiletest.Data(t)
	m := proof.Manifest{Profiles: map[string]*model.ProfileData{profiletest.Name: data}}
	entry := proof.Entry{ID: "text.dashes", Code: "em-dash", Red: "r", Near: "n", Profile: profiletest.Name}
	p := m.PolicyOf(entry)
	if p.Profile != profiletest.Name || p.ProfileData != data || p.Digest != proof.PolicyFor(entry).Digest {
		t.Errorf("PolicyOf(profile entry) = %+v", p)
	}
	entry.Profile = "other"
	if p := m.PolicyOf(entry); p.Profile != "other" || p.ProfileData != nil {
		t.Errorf("PolicyOf(unknown profile) = %+v", p)
	}
	entry.Profile = ""
	if p := m.PolicyOf(entry); p.Profile != "" || p.ProfileData != nil {
		t.Errorf("PolicyOf(core entry) = %+v", p)
	}
	if p := (proof.Manifest{}).PolicyOf(proof.Entry{Profile: profiletest.Name}); p.ProfileData != nil {
		t.Errorf("a manifest without profiles carries no data: %+v", p)
	}
}

// TestTheProfileManifestJoinsTheProof pins the fifth manifest: it is read
// strictly beside the four of M1 to M3, every entry of it runs under the
// fixture profile, and the production proof holds its data for that name.
func TestTheProfileManifestJoinsTheProof(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifest("profile.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range m.Checks {
		if e.Profile != profiletest.Name {
			t.Errorf("%s runs under %q, want the fixture profile", e.ID, e.Profile)
		}
	}
	all := production(t)
	if all.Profiles[profiletest.Name] == nil || len(all.Names()) < len(m.Names()) {
		t.Errorf("the production proof carries no data for %s", profiletest.Name)
	}
}
