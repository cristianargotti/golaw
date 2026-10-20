package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

type setupDiagnosticCheck struct {
	item setupAssessment
}

func (c setupDiagnosticCheck) Spec() model.Spec {
	return model.Spec{
		ID: c.item.id, Rule: "rules/5.6@1",
		Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: c.item.id,
	}
}

func (c setupDiagnosticCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, err := in.Snapshot.Read("input.json")
	if err != nil {
		return model.Result{}, fmt.Errorf("read setup diagnostic fixture: %w", err)
	}
	facts := setupBaseline()
	if err := json.Unmarshal(data, &facts); err != nil {
		return model.Result{}, fmt.Errorf("decode setup diagnostic fixture: %w", err)
	}
	result := model.Result{Status: model.Measured(), Scope: c.item.id}
	for _, line := range c.item.run(facts) {
		if line.State == "ok" || line.State == "not-armed" || line.State == "disabled" {
			continue
		}
		result.Findings = append(result.Findings, model.Finding{
			Code: line.Code, Message: line.Detail, Severity: model.SeverityNote,
		})
	}
	return result, nil
}

func registerSetupProof(t *testing.T, builder *engine.Builder) {
	t.Helper()
	for _, item := range setupAssessments() {
		if err := builder.Register(setupDiagnosticCheck{item: item}); err != nil {
			t.Fatal(err)
		}
	}
}
