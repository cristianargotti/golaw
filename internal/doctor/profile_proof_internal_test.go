package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// profileDiagnosticCheck proves one red code of the profile line both
// ways: its fixture is the evidence the binding collects about the pinned
// bundle, as JSON, and it reports its own code alone, so the deletion pass
// of each code depends on its own entry.
type profileDiagnosticCheck struct{ id, code string }

// profileProofs are the proof entries of the profile line, one per red code.
func profileProofs() []profileDiagnosticCheck {
	return []profileDiagnosticCheck{
		{id: idProfile, code: codeProfileNotInstalled},
		{id: "doctor." + codeProfileVersionMismatch, code: codeProfileVersionMismatch},
		{id: "doctor." + codeProfileDigestMismatch, code: codeProfileDigestMismatch},
		{id: "doctor." + codeProfileStale, code: codeProfileStale},
		{id: "doctor." + codeProfileDataInvalid, code: codeProfileDataInvalid},
	}
}

func (c profileDiagnosticCheck) Spec() model.Spec {
	return model.Spec{
		ID: c.id, Rule: "rules/5.6@1",
		Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: c.id,
	}
}

func (c profileDiagnosticCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, err := in.Snapshot.Read("input.json")
	if err != nil {
		return model.Result{}, fmt.Errorf("read profile diagnostic fixture: %w", err)
	}
	var facts ProfileFacts
	if err := json.Unmarshal(data, &facts); err != nil {
		return model.Result{}, fmt.Errorf("decode profile diagnostic fixture: %w", err)
	}
	result := model.Result{Status: model.Measured(), Scope: c.id}
	if line := AssessProfile(facts); line.Code == c.code {
		result.Findings = append(result.Findings, model.Finding{
			Code: line.Code, Message: line.Detail, Severity: model.SeverityNote,
		})
	}
	return result, nil
}

func registerProfileProof(t *testing.T, builder *engine.Builder) {
	t.Helper()
	for _, c := range profileProofs() {
		if err := builder.Register(c); err != nil {
			t.Fatal(err)
		}
	}
}
