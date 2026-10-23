package proof_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestEveryConditionedCheckOfTheCatalogAppliesOnlyWhereItsCapabilityHolds
// binds each check of the catalog that declares a capability to its red
// fixture with and without that capability: every goast check needs go and
// goast.raw-error-response also api, and none of them judges a repository
// where its capability is off.
func TestEveryConditionedCheckOfTheCatalogAppliesOnlyWhereItsCapabilityHolds(t *testing.T) {
	t.Parallel()
	m := production(t)
	reg, err := checks.Registry(engine.WithProofs(m.Names()))
	if err != nil {
		t.Fatal(err)
	}
	var conditioned []string
	for _, c := range reg.Checks() {
		spec := c.Spec()
		if strings.HasPrefix(spec.ID, "goast.") && !slices.Contains(spec.Needs, "go") {
			t.Errorf("%s needs %v, want go among them", spec.ID, spec.Needs)
		}
		if len(spec.Needs) > 0 {
			conditioned = append(conditioned, spec.ID)
		}
	}
	if raw, ok := reg.Lookup("goast.raw-error-response"); !ok || !slices.Equal(raw.Spec().Needs, []string{"go", "api"}) {
		t.Errorf("goast.raw-error-response is not conditioned by go and api")
	}
	if len(conditioned) < 17 {
		t.Errorf("conditioned checks = %v, want at least the 17 of the goast family", conditioned)
	}
	if failures := proof.Capability(t.Context(), reg, ".", m); len(failures) != 0 {
		t.Fatalf("the capability pass does not hold:\n%s", reasons(failures))
	}
}
