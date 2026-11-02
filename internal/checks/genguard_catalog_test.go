package checks_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/checks/genguard"
)

func TestTheCatalogCarriesTheGenFamily(t *testing.T) {
	t.Parallel()
	ids := map[string]bool{}
	for _, c := range checks.Catalog() {
		ids[c.Spec().ID] = true
	}
	for _, c := range genguard.Checks() {
		if !ids[c.Spec().ID] {
			t.Errorf("the catalog lacks %s", c.Spec().ID)
		}
	}
	if _, err := checks.Registry(); err != nil {
		t.Fatalf("the production registry refuses the family: %v", err)
	}
}
