package proof_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestProductionCatalogIsProven is the proof of section 10.2 over the real
// catalog: every registered check has a manifest entry and every entry a
// registered check, the red fixture of each check yields its code through
// production dispatch, the near miss does not, deleting the check turns the
// red case green and over-firing it turns the near miss red.
func TestProductionCatalogIsProven(t *testing.T) {
	t.Parallel()
	m, err := proof.LoadManifest("manifest.yml")
	if err != nil {
		t.Fatal(err)
	}
	reg, err := checks.Registry(engine.WithProofs(m.Names()))
	if err != nil {
		t.Fatal(err)
	}
	if failures := proof.Verify(t.Context(), reg, ".", m); len(failures) != 0 {
		t.Fatalf("the proof does not hold:\n%s", reasons(failures))
	}
}
