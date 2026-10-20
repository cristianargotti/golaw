package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// managedDiagnosticCheck proves doctor.managed both ways: its fixture is
// the evidence CollectManaged gathers, as JSON.
type managedDiagnosticCheck struct{}

func (managedDiagnosticCheck) Spec() model.Spec {
	return model.Spec{
		ID: idManaged, Rule: "rules/5.6@1",
		Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: idManaged,
	}
}

func (managedDiagnosticCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, err := in.Snapshot.Read("input.json")
	if err != nil {
		return model.Result{}, fmt.Errorf("read managed diagnostic fixture: %w", err)
	}
	var facts []ManagedFact
	if err := json.Unmarshal(data, &facts); err != nil {
		return model.Result{}, fmt.Errorf("decode managed diagnostic fixture: %w", err)
	}
	result := model.Result{Status: model.Measured(), Scope: idManaged}
	for _, line := range AssessManaged(facts) {
		if line.State == stateOK {
			continue
		}
		result.Findings = append(result.Findings, model.Finding{
			Code: line.Code, Message: line.Detail, Severity: model.SeverityNote,
		})
	}
	return result, nil
}

func registerManagedProof(t *testing.T, builder *engine.Builder) {
	t.Helper()
	if err := builder.Register(managedDiagnosticCheck{}); err != nil {
		t.Fatal(err)
	}
}
