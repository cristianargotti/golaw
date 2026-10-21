package checks_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestEveryFamilyRunsWhereItsConditionHolds is the applicability matrix of
// the catalog: every family of section 6 and every policy check runs, with
// and without its capability, exactly in the scans, the commit rung and the
// hook shapes its scope gives it (section 1.2 and D1).
func TestEveryFamilyRunsWhereItsConditionHolds(t *testing.T) {
	t.Parallel()
	reg, err := checks.Registry()
	if err != nil {
		t.Fatal(err)
	}
	failures := proof.Matrix(reg)
	lines := make([]string, 0, len(failures))
	for _, f := range failures {
		lines = append(lines, f.String())
	}
	if len(lines) != 0 {
		t.Fatalf("the applicability matrix does not hold:\n%s", strings.Join(lines, "\n"))
	}
	var families []string
	for _, id := range reg.IDs() {
		family, _, _ := strings.Cut(id, ".")
		if !slices.Contains(families, family) {
			families = append(families, family)
		}
	}
	for _, want := range []string{"text", "paths", "goast", "shell", "commit"} {
		if !slices.Contains(families, want) {
			t.Errorf("the catalog holds no %s check: families %v", want, families)
		}
	}
}
