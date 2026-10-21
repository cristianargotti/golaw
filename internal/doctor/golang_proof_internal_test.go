package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// goDiagnosticCheck proves doctor.go-toolchain both ways: its fixture is
// the evidence the binding collects, as JSON.
type goDiagnosticCheck struct{}

func (goDiagnosticCheck) Spec() model.Spec {
	return model.Spec{
		ID: idGoToolchain, Rule: "rules/5.6@1",
		Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: idGoToolchain,
	}
}

func (goDiagnosticCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, err := in.Snapshot.Read("input.json")
	if err != nil {
		return model.Result{}, fmt.Errorf("read go diagnostic fixture: %w", err)
	}
	var facts GoFacts
	if err := json.Unmarshal(data, &facts); err != nil {
		return model.Result{}, fmt.Errorf("decode go diagnostic fixture: %w", err)
	}
	result := model.Result{Status: model.Measured(), Scope: idGoToolchain}
	for _, line := range AssessGo(facts) {
		if line.State == stateOK {
			continue
		}
		result.Findings = append(result.Findings, model.Finding{
			Code: line.Code, Message: line.Detail, Severity: model.SeverityNote,
		})
	}
	return result, nil
}

func registerGoProof(t *testing.T, builder *engine.Builder) {
	t.Helper()
	if err := builder.Register(goDiagnosticCheck{}); err != nil {
		t.Fatal(err)
	}
}
