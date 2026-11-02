package genguard_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/genguard"
)

func TestEveryCheckOfTheFamilyIsAGenCheck(t *testing.T) {
	t.Parallel()
	got := genguard.Checks()
	if got == nil {
		t.Fatal("Checks() = nil, want a list the catalog can join")
	}
	for _, c := range got {
		if spec := c.Spec(); !strings.HasPrefix(spec.ID, "gen.") || spec.Proof != spec.ID {
			t.Errorf("check %q with proof %q is outside the gen family", spec.ID, spec.Proof)
		}
	}
}
