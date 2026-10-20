package coverage_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type coverageProofCheck struct {
	t *testing.T
}

func (coverageProofCheck) Spec() model.Spec {
	return model.Spec{
		ID: "coverage.floors", Rule: "rules/3@1",
		Scope: model.ScopeTree, Owner: model.OwnerCore,
		Proof: "coverage.floors",
	}
}

func (c coverageProofCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	root := c.t.TempDir()
	for _, file := range in.Snapshot.Files() {
		data, err := in.Snapshot.Read(file.Path)
		if err != nil {
			return model.Result{}, fmt.Errorf("read coverage fixture: %w", err)
		}
		writeCoverageFile(c.t, root, file.Path, string(data))
	}
	data, err := in.Snapshot.Read("profile.cov")
	if err != nil {
		return model.Result{}, fmt.Errorf("read profile fixture: %w", err)
	}
	report, err := coverage.Evaluate(bytes.NewReader(data), root, coverage.DefaultFloors())
	if err != nil {
		return model.Result{}, fmt.Errorf("evaluate coverage fixture: %w", err)
	}
	result := model.Result{Status: model.Measured(), Scope: "coverage.floors"}
	for _, failure := range report.Failures {
		result.Findings = append(result.Findings, model.Finding{
			Code: "coverage-floor", Severity: model.SeverityFail,
			Message: "coverage below floor: " + failure.Name,
		})
	}
	return result, nil
}

func TestCoverageManifestProof(t *testing.T) {
	t.Parallel()
	manifest, err := proof.LoadManifest("proof/manifest.yml")
	if err != nil {
		t.Fatal(err)
	}
	builder := engine.New()
	if registerErr := builder.Register(coverageProofCheck{t: t}); registerErr != nil {
		t.Fatal(registerErr)
	}
	registry, err := builder.Build(engine.WithProofs(manifest.Names()))
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range proof.Verify(t.Context(), registry, "proof", manifest) {
		t.Error(failure.String())
	}
}
