package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestAssessGenerateSaysWhichBinaryTheHookCanRun(t *testing.T) {
	t.Parallel()
	facts := GenerateFacts{Tools: []GenerateTool{
		{Name: "golangci-lint", Version: "2.14.0", State: "verified", Path: "/data/golaw/tools/bin/golangci-lint/2.14.0/linux-amd64/golangci-lint"},
		{Name: "sqlc", Version: "1.31.1", State: "missing", Detail: "sqlc 1.31.1 is not installed"},
		{Name: "oapi-codegen", Version: "2.8.0", State: "drift", Path: "/data/golaw/tools/bin/oapi-codegen/2.8.0/linux-amd64/oapi-codegen"},
		{Name: "goverter", Version: "1.11.1", State: "path", Path: "/usr/local/bin/goverter"},
	}}
	want := []Line{
		{ID: "doctor.gen-tools", Key: "golangci-lint", State: "ok", Code: "gen-tool-available", Detail: "2.14.0 /data/golaw/tools/bin/golangci-lint/2.14.0/linux-amd64/golangci-lint (sha256 verified)"},
		{ID: "doctor.gen-tools", Key: "sqlc", State: "missing", Code: "gen-tool-missing", Detail: "1.31.1: not built on this machine: run golaw setup --gen; golaw gen and the pipeline still run it"},
		{ID: "doctor.gen-tools", Key: "oapi-codegen", State: "drift", Code: "gen-tool-drift", Detail: "2.8.0 at /data/golaw/tools/bin/oapi-codegen/2.8.0/linux-amd64/oapi-codegen no longer matches its receipt: run golaw setup --gen"},
		{ID: "doctor.gen-tools", Key: "goverter", State: "ok", Code: "gen-tool-available", Detail: "1.11.1 /usr/local/bin/goverter (on PATH, version matches, checksum not verified)"},
	}
	if got := AssessGenerate(facts); !reflect.DeepEqual(got, want) {
		t.Errorf("AssessGenerate =\n%+v\nwant\n%+v", got, want)
	}
	for _, line := range AssessGenerate(facts) {
		if line.State == stateRed {
			t.Errorf("red line %+v: a missing generator is never red (M6.5.3)", line)
		}
	}
	if got := AssessGenerate(GenerateFacts{}); len(got) != 0 {
		t.Errorf("no tool = %+v", got)
	}
}

// generateDiagnosticCheck proves one generator doctor code both ways: its
// fixture is the GenerateFacts the binding collects, as JSON.
type generateDiagnosticCheck struct{ id, code string }

func (c generateDiagnosticCheck) Spec() model.Spec {
	return model.Spec{ID: c.id, Rule: "rules/5.6@1", Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: c.id}
}

func (c generateDiagnosticCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, err := in.Snapshot.Read("input.json")
	if err != nil {
		return model.Result{}, fmt.Errorf("read generator diagnostic fixture: %w", err)
	}
	var facts GenerateFacts
	if err := json.Unmarshal(data, &facts); err != nil {
		return model.Result{}, fmt.Errorf("decode generator diagnostic fixture: %w", err)
	}
	result := model.Result{Status: model.Measured(), Scope: c.id}
	for _, line := range AssessGenerate(facts) {
		if line.Code == c.code {
			result.Findings = append(result.Findings, model.Finding{Code: line.Code, Message: line.Detail, Severity: model.SeverityNote})
		}
	}
	return result, nil
}

func registerGenerateProof(t *testing.T, builder *engine.Builder) {
	t.Helper()
	for _, c := range []generateDiagnosticCheck{
		{"doctor.gen-tool-missing", "gen-tool-missing"},
		{"doctor.gen-tool-drift", "gen-tool-drift"},
	} {
		if err := builder.Register(c); err != nil {
			t.Fatal(err)
		}
	}
}
