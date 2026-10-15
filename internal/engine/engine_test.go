package engine_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// fake is a check whose behavior a test chooses.
type fake struct {
	spec model.Spec
	run  func(ctx context.Context, in model.Input) (model.Result, error)
}

func (f fake) Spec() model.Spec { return f.spec }

func (f fake) Run(ctx context.Context, in model.Input) (model.Result, error) {
	if f.run == nil {
		return model.Result{Status: model.Measured(), Scope: "nothing"}, nil
	}
	return f.run(ctx, in)
}

func spec(id string, scope model.Scope) model.Spec {
	return model.Spec{ID: id, Rule: "rules/1@1", Scope: scope, Owner: model.OwnerCore, Proof: id}
}

func build(t *testing.T, checks ...model.Check) *engine.Registry {
	t.Helper()
	b := engine.New()
	for _, c := range checks {
		if err := b.Register(c); err != nil {
			t.Fatal(err)
		}
	}
	reg, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func TestRegisterRefusesNilInvalidAndDuplicate(t *testing.T) {
	t.Parallel()
	b := engine.New()
	if err := b.Register(nil); !errors.Is(err, model.ErrInvalidSpec) {
		t.Errorf("nil check: %v", err)
	}
	if err := b.Register(fake{spec: model.Spec{ID: "bad"}}); !errors.Is(err, model.ErrInvalidSpec) {
		t.Errorf("invalid spec: %v", err)
	}
	if err := b.Register(fake{spec: spec("text.dashes", model.ScopeFile)}); err != nil {
		t.Fatal(err)
	}
	if err := b.Register(fake{spec: spec("text.dashes", model.ScopeTree)}); !errors.Is(err, model.ErrDuplicateCheck) {
		t.Errorf("duplicate: %v", err)
	}
}

func TestBuildOrdersByID(t *testing.T) {
	t.Parallel()
	reg := build(t,
		fake{spec: spec("text.todo", model.ScopeFile)},
		fake{spec: spec("goast.file-length", model.ScopeFile)},
		fake{spec: spec("shell.no-verify", model.ScopeEvent)},
	)
	if got := reg.String(); got != "goast.file-length,shell.no-verify,text.todo" {
		t.Errorf("IDs = %s", got)
	}
	if reg.Len() != 3 {
		t.Errorf("Len = %d", reg.Len())
	}
	if _, ok := reg.Lookup("shell.no-verify"); !ok {
		t.Error("Lookup must find a registered check")
	}
	if _, ok := reg.Lookup("shell.push"); ok {
		t.Error("Lookup must not find an unregistered check")
	}
	checks := reg.Checks()
	checks[0] = nil
	if _, ok := reg.Lookup("goast.file-length"); !ok || reg.Checks()[0] == nil {
		t.Error("Checks must return a copy")
	}
}

func TestBuildWithProofs(t *testing.T) {
	t.Parallel()
	b := engine.New()
	if err := b.Register(fake{spec: spec("text.dashes", model.ScopeFile)}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Build(engine.WithProofs([]string{"text.todo"})); !errors.Is(err, model.ErrUnknownProof) {
		t.Errorf("unknown proof: %v", err)
	}
	if _, err := b.Build(engine.WithProofs([]string{"text.dashes"})); err != nil {
		t.Errorf("known proof: %v", err)
	}
	if _, err := b.Build(); err != nil {
		t.Errorf("without the option: %v", err)
	}
}
