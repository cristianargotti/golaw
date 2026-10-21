package doctor

import (
	"context"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type diagnosticCheck struct {
	item assessment
	base Snapshot
}

func (c diagnosticCheck) Spec() model.Spec {
	return model.Spec{
		ID: c.item.id, Rule: "rules/5.6@1",
		Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: c.item.id,
	}
}

func (c diagnosticCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, err := in.Snapshot.Read("input.json")
	if err != nil {
		return model.Result{}, fmt.Errorf("read diagnostic fixture: %w", err)
	}
	snapshot, err := patched(c.base, data)
	if err != nil {
		return model.Result{}, fmt.Errorf("decode diagnostic fixture: %w", err)
	}
	result := model.Result{Status: model.Measured(), Scope: c.item.id}
	for _, line := range c.item.run(snapshot) {
		if line.State == "ok" || line.State == "source-build" {
			continue
		}
		result.Findings = append(result.Findings, model.Finding{
			Code: line.Code, Message: line.Detail, Severity: model.SeverityNote,
		})
	}
	return result, nil
}

func TestDoctorProof(t *testing.T) {
	t.Parallel()
	manifest, err := proof.LoadManifest("proof/manifest.yml")
	if err != nil {
		t.Fatal(err)
	}
	builder := engine.New()
	for _, item := range assessments() {
		err = builder.Register(diagnosticCheck{item: item, base: baseline(t)})
		if err != nil {
			t.Fatal(err)
		}
	}
	registerSetupProof(t, builder)
	registerManagedProof(t, builder)
	registerGoProof(t, builder)
	registry, err := builder.Build(engine.WithProofs(manifest.Names()))
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range proof.Verify(t.Context(), registry, "proof", manifest) {
		t.Error(failure.String())
	}
}
