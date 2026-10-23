package proof_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestProductionCatalogIsProven is the proof of section 10.2 over the real
// catalog and the four manifests (the families of M1 and M2, then the lint,
// build and dependency halves of the policy family): every registered
// check has an entry and every entry a registered check, the red fixture of
// each check yields its code through production dispatch, the near miss
// does not, deleting the check turns the red case green and over-firing it
// turns the near miss red.
func TestProductionCatalogIsProven(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifests("manifest.yml", "policyfiles-lint.yml", "policyfiles-build.yml", "policyfiles-deps.yml")
	if err != nil {
		t.Fatal(err)
	}
	reg, err := proof.Registry(checks.Catalog(), m)
	if err != nil {
		t.Fatal(err)
	}
	if failures := proof.Verify(t.Context(), reg, ".", m); len(failures) != 0 {
		t.Fatalf("the proof does not hold:\n%s", reasons(failures))
	}
}
