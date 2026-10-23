package proof_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestRegistryAdmitsOnlyProvenChecks holds the registry the proof verifies
// to its manifest: a check the manifest names enters it, while a check
// registered twice or one without a proof pair is refused with a reason
// that names the check, so no check enters the proven catalog unproven.
func TestRegistryAdmitsOnlyProvenChecks(t *testing.T) {
	t.Parallel()
	m := proof.Manifest{Checks: []proof.Entry{{ID: "text.dashes"}}}
	reg, err := proof.Registry([]model.Check{dashCheck{id: "text.dashes"}}, m)
	if err != nil || reg.String() != "text.dashes" {
		t.Fatalf("a proven check = %v, %v", reg, err)
	}
	twice := []model.Check{dashCheck{id: "text.dashes"}, dashCheck{id: "text.dashes"}}
	if _, err := proof.Registry(twice, m); !errors.Is(err, model.ErrDuplicateCheck) || !strings.Contains(err.Error(), "proof: register text.dashes") {
		t.Errorf("a check registered twice = %v", err)
	}
	unproven := []model.Check{dashCheck{id: "text.dashes"}, noVerifyCheck{}}
	if _, err := proof.Registry(unproven, m); !errors.Is(err, model.ErrUnknownProof) || !strings.Contains(err.Error(), "proof: build the registry") {
		t.Errorf("a check without a proof pair = %v", err)
	}
}
