package policyfiles_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// TestPolicyOfCarriesTheProfileData pins the test seam of D6: a fixture
// with data measures under that data with the profile of the data
// enrolled, a named profile wins over the data's name, and a fixture
// without data stays core only.
func TestPolicyOfCarriesTheProfileData(t *testing.T) {
	t.Parallel()
	data := profiletest.Data(t)
	check := policyfiles.HookTests()
	if p := policyOf(check, fixture{data: data}); p.Profile != profiletest.Name || p.ProfileData != data {
		t.Errorf("policyOf(data) = profile %q, data %+v", p.Profile, p.ProfileData)
	}
	if p := policyOf(check, fixture{profile: "other", data: data}); p.Profile != "other" || p.ProfileData != data {
		t.Errorf("a named profile wins: %q", p.Profile)
	}
	if p := policyOf(check, fixture{}); p.Profile != "" || p.ProfileData != nil {
		t.Errorf("core only: %+v", p)
	}
}
