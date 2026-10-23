package proof_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// scopedCheck is a check of any id, scope, owner, needs and requires that
// measures nothing; the matrix reads its specification alone.
type scopedCheck struct {
	id       string
	scope    model.Scope
	owner    string
	needs    []string
	requires []string
}

func (s scopedCheck) Spec() model.Spec {
	owner := s.owner
	if owner == "" {
		owner = model.OwnerCore
	}
	return model.Spec{ID: s.id, Rule: "rules/4.1@1", Scope: s.scope, Owner: owner, Needs: s.needs, Requires: s.requires, Proof: s.id}
}

func (scopedCheck) Run(context.Context, model.Input) (model.Result, error) {
	return model.Result{Status: model.Measured()}, nil
}

func TestMatrixHoldsForChecksThatDeclareTheirCondition(t *testing.T) {
	t.Parallel()
	reg := registry(t,
		scopedCheck{id: "text.dashes", scope: model.ScopeFile},
		scopedCheck{id: "paths.secret-material", scope: model.ScopeEvent},
		scopedCheck{id: "goast.file-length", scope: model.ScopeFile, needs: []string{"go"}},
		scopedCheck{id: "goast.raw-error-response", scope: model.ScopeFile, needs: []string{"api", "go"}},
		scopedCheck{id: "shell.no-verify", scope: model.ScopeEvent},
		scopedCheck{id: "commit.conventional", scope: model.ScopeCommit},
		scopedCheck{id: "policy.image-digest", scope: model.ScopeTree, needs: []string{"image"}},
		scopedCheck{id: "policy.gitleaks-generic", scope: model.ScopeTree},
		scopedCheck{id: "text.profiled", scope: model.ScopeFile, owner: "acme"},
	)
	if failures := proof.Matrix(reg); len(failures) != 0 {
		t.Fatalf("Matrix() = \n%s", reasons(failures))
	}
}

func TestMatrixRefusesACheckOffItsCondition(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		check model.Check
		want  string
	}{
		"a need the family lacks":    {scopedCheck{id: "text.dashes", scope: model.ScopeFile, needs: []string{"go"}}, "declares scope file and needs [go], its condition is scope file and needs []"},
		"a dropped need":             {scopedCheck{id: "goast.file-length", scope: model.ScopeFile}, "declares scope file and needs [], its condition is scope file and needs [go]"},
		"a policy check in a hook":   {scopedCheck{id: "policy.makefile", scope: model.ScopeFile, needs: []string{"go"}}, "declares scope file and needs [go], its condition is scope tree and needs [go]"},
		"the wrong capability":       {scopedCheck{id: "policy.squawk-config", scope: model.ScopeTree, needs: []string{"api"}}, "its condition is scope tree and needs [db]"},
		"a family without condition": {scopedCheck{id: "web.thing", scope: model.ScopeFile}, "no condition: write the scope and the needs of the check in internal/proof/conditions_export_test.go"},
		"a policy check without row": {scopedCheck{id: "policy.other", scope: model.ScopeTree}, "no condition"},
		"a need the engine cannot meet": {
			scopedCheck{id: "text.todo", scope: model.ScopeFile, requires: []string{"external-evidence"}},
			"in scan under [api cli db go hexagonal image] the engine runs it: false, its condition says true",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			failures := proof.Matrix(registry(t, tc.check))
			if len(failures) == 0 || failures[0].Pass != proof.PassCondition || failures[0].ID != tc.check.Spec().ID || !strings.Contains(failures[0].Reason, tc.want) {
				t.Fatalf("Matrix() = %v, want %q", failures, tc.want)
			}
		})
	}
}

func TestConditionOfReadsTheRowOfTheCheckThenOfItsFamily(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]model.Scope{"text.anything": model.ScopeFile, "paths.secret-material": model.ScopeEvent, "policy.ci-target": model.ScopeTree, "commit.shape": model.ScopeCommit} {
		if got, ok := proof.ConditionOf(id); !ok || got.Scope != want {
			t.Errorf("ConditionOf(%s) = %+v, %v", id, got, ok)
		}
	}
	if _, ok := proof.ConditionOf("coverage.floors"); ok {
		t.Error("coverage.floors is no dispatched check and has no condition")
	}
}
