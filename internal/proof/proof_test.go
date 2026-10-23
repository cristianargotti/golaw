package proof_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profiletest"
	"github.com/cristianargotti/golaw/internal/proof"
)

// production is the whole proof of the kit: the five manifests (the
// families of M1 and M2, the lint, build and dependency halves of the
// policy family, and the checks the profile data turns on) with the data
// of the fixture profile for the entries that name it (D6).
func production(t *testing.T) proof.Manifest {
	t.Helper()
	m, err := proof.LoadManifests("manifest.yml", "policyfiles-lint.yml", "policyfiles-build.yml", "policyfiles-deps.yml", "profile.yml")
	if err != nil {
		t.Fatal(err)
	}
	m.Profiles = map[string]*model.ProfileData{profiletest.Name: profiletest.Data(t)}
	return m
}

// TestProductionCatalogIsProven is the proof of section 10.2 over the real
// catalog and the five manifests: every registered check has an entry and
// every entry a registered check, the red fixture of each check yields its
// code through production dispatch, the near miss does not, deleting the
// check turns the red case green and over-firing it turns the near miss
// red.
func TestProductionCatalogIsProven(t *testing.T) {
	t.Parallel()
	m := production(t)
	reg, err := proof.Registry(checks.Catalog(), m)
	if err != nil {
		t.Fatal(err)
	}
	if failures := proof.Verify(t.Context(), reg, ".", m); len(failures) != 0 {
		t.Fatalf("the proof does not hold:\n%s", reasons(failures))
	}
}
