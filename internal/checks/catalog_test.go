package checks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
	checksurface "github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

type stub struct {
	id string
}

func (s stub) Spec() model.Spec {
	return model.Spec{ID: s.id, Rule: "rules/1@1", Scope: model.ScopeFile, Owner: model.OwnerCore, Proof: s.id}
}

func (stub) Run(context.Context, model.Input) (model.Result, error) {
	return model.Result{Status: model.Measured()}, nil
}

func TestRegistryBuildsTheCatalog(t *testing.T) {
	t.Parallel()
	reg, err := checks.Registry()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Len() != len(checks.Catalog()) {
		t.Errorf("Registry() holds %d checks, the catalog %d", reg.Len(), len(checks.Catalog()))
	}
	for _, c := range checks.Catalog() {
		if err := c.Spec().Validate(); err != nil {
			t.Errorf("catalog check %s: %v", c.Spec().ID, err)
		}
	}
}

func TestCatalogHoldsTheSurfaceFamily(t *testing.T) {
	t.Parallel()
	ids := map[string]bool{}
	for _, c := range checks.Catalog() {
		ids[c.Spec().ID] = true
	}
	for _, c := range checksurface.Checks() {
		if !ids[c.Spec().ID] {
			t.Errorf("the catalog lacks %s", c.Spec().ID)
		}
	}
}

func TestBuildRefusesDuplicatesAndUnknownProofs(t *testing.T) {
	t.Parallel()
	if _, err := checks.Build([]model.Check{stub{id: "text.a"}, stub{id: "text.a"}}); !errors.Is(err, model.ErrDuplicateCheck) {
		t.Errorf("duplicate: %v", err)
	}
	if _, err := checks.Build([]model.Check{stub{id: "text.a"}}, engine.WithProofs([]string{"text.b"})); !errors.Is(err, model.ErrUnknownProof) {
		t.Errorf("unknown proof: %v", err)
	}
	reg, err := checks.Build([]model.Check{stub{id: "text.b"}, stub{id: "text.a"}})
	if err != nil || reg.String() != "text.a,text.b" {
		t.Errorf("Build() = %s, %v", reg, err)
	}
}
