package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/release"
)

// releaseFixture is what a person downloaded and what doctor hashed, as
// text, so a fixture reads as the files it stands for.
type releaseFixture struct {
	Asset        string `json:"asset"`
	Version      string `json:"version"`
	BinaryDigest string `json:"binary_digest"`
	Checksums    string `json:"checksums"`
	Bundle       string `json:"bundle"`
	Attestation  string `json:"attestation"`
}

// releaseDiagnosticCheck proves one red code of the release line both
// ways: the fixture runs through release.Verify and AssessRelease, as the
// binding runs them, and it reports its own code alone, so the deletion
// pass of each code depends on its own entry.
type releaseDiagnosticCheck struct{ id, code string }

// releaseProofs are the proof entries of the release line, one per red code.
func releaseProofs() []releaseDiagnosticCheck {
	return []releaseDiagnosticCheck{{id: idRelease, code: release.CodeChecksumMismatch}}
}

func (c releaseDiagnosticCheck) Spec() model.Spec {
	return model.Spec{
		ID: c.id, Rule: "rules/5.6@1",
		Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: c.id,
	}
}

func (c releaseDiagnosticCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	data, err := in.Snapshot.Read("input.json")
	if err != nil {
		return model.Result{}, fmt.Errorf("read release diagnostic fixture: %w", err)
	}
	var f releaseFixture
	if err := json.Unmarshal(data, &f); err != nil {
		return model.Result{}, fmt.Errorf("decode release diagnostic fixture: %w", err)
	}
	verdict := release.Verify(release.Inputs{
		BinaryDigest: f.BinaryDigest, Asset: f.Asset, Version: f.Version,
		Checksums: []byte(f.Checksums), Bundle: []byte(f.Bundle), Attestation: []byte(f.Attestation),
	})
	result := model.Result{Status: model.Measured(), Scope: c.id}
	if line := AssessRelease(f.Asset, verdict); line.Code == c.code {
		result.Findings = append(result.Findings, model.Finding{
			Code: line.Code, Message: line.Detail, Severity: model.SeverityNote,
		})
	}
	return result, nil
}

func registerReleaseProof(t *testing.T, builder *engine.Builder) {
	t.Helper()
	for _, c := range releaseProofs() {
		if err := builder.Register(c); err != nil {
			t.Fatal(err)
		}
	}
}
