package policyfiles_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

// needsDB is a tree check shaped like the family's that needs db, so the
// tests of the shared helper see applicability decide before Run does.
type needsDB struct{}

func (needsDB) Spec() model.Spec {
	return model.Spec{
		ID: "policy.stub", Rule: "rules/4.9@1", Scope: model.ScopeTree,
		Owner: model.OwnerCore, Needs: []string{"db"}, Proof: "policy.stub",
	}
}

func (needsDB) Run(_ context.Context, in model.Input) (model.Result, error) {
	res := model.Result{Status: model.Measured(), Scope: "1 file"}
	if _, err := in.Snapshot.Read("schema.sql"); err == nil {
		res.Findings = []model.Finding{{Code: "stub-finding", Path: "schema.sql", Message: "a stub finding", Severity: model.SeverityDeny}}
	}
	return res, nil
}

// TestMeasureHoldsApplicabilityByCapabilityAndScope pins the shared helper:
// the check runs only where its capability holds, a tree check runs over a
// scan and over the index, and the result is the one production records.
func TestMeasureHoldsApplicabilityByCapabilityAndScope(t *testing.T) {
	t.Parallel()
	files := map[string]string{"schema.sql": "create table t (id int);\n"}
	if res := measure(t, needsDB{}, fixture{files: files, capabilities: []string{"go"}}); res.Status.Kind != model.StatusNotApplicable ||
		res.Status.Reason != "capability db is not enabled" {
		t.Errorf("without db = %+v", res)
	}
	for _, mode := range []model.Mode{model.ModeScan, model.ModeStaged} {
		res := measure(t, needsDB{}, fixture{files: files, capabilities: []string{"go", "db"}, mode: mode})
		if res.Status.Kind != model.StatusMeasured || !slices.Equal(codes(res), []string{"stub-finding"}) {
			t.Errorf("%s with db = %+v", mode, res)
		}
	}
	if res := measure(t, needsDB{}, fixture{capabilities: []string{"go", "db"}}); len(codes(res)) != 0 {
		t.Errorf("an empty tree = %+v", res)
	}
}

// TestEveryCheckOfTheFamilyIsATreeCheck holds every check any half
// registers to contract D1 and D4: an id under policy., the tree scope,
// the core owner, its id as its proof key and a rule the model accepts.
func TestEveryCheckOfTheFamilyIsATreeCheck(t *testing.T) {
	t.Parallel()
	for _, c := range policyfiles.Checks() {
		spec := c.Spec()
		if err := spec.Validate(); err != nil || spec.Scope != model.ScopeTree || spec.Owner != model.OwnerCore ||
			!strings.HasPrefix(spec.ID, "policy.") || spec.Proof != spec.ID {
			t.Errorf("%s = %+v (%v)", spec.ID, spec, err)
		}
	}
}
