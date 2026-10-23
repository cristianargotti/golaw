package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/release"
	"github.com/cristianargotti/golaw/internal/releasetest"
)

// releaseFixture is what a person downloaded and what doctor hashed, as
// text, so a fixture reads as the files it stands for; or, for a code a
// signature decides, the name of a release the proof generated (good or
// one tamper of releasetest), its trust root left out with "none".
type releaseFixture struct {
	Asset        string `json:"asset"`
	Version      string `json:"version"`
	BinaryDigest string `json:"binary_digest"`
	Checksums    string `json:"checksums"`
	Bundle       string `json:"bundle"`
	Attestation  string `json:"attestation"`
	Material     string `json:"release_material"`
	Trust        string `json:"release_trust"`
}

// releaseDiagnosticCheck proves one red code of the release line both
// ways: the fixture runs through release.Verify and AssessRelease, as the
// binding runs them, and it reports its own code alone, so the deletion
// pass of each code depends on its own entry.
type releaseDiagnosticCheck struct {
	id, code  string
	materials map[string]release.Inputs
}

// releaseProofs are the proof entries of the release line, one per red code.
func releaseProofs() []releaseDiagnosticCheck {
	return []releaseDiagnosticCheck{
		{id: idRelease, code: release.CodeChecksumMismatch},
		{id: "doctor." + release.CodeInputMissing, code: release.CodeInputMissing},
		{id: "doctor." + release.CodeTrustRootInvalid, code: release.CodeTrustRootInvalid},
		{id: "doctor." + release.CodeSignatureInvalid, code: release.CodeSignatureInvalid},
		{id: "doctor." + release.CodeIdentityMismatch, code: release.CodeIdentityMismatch},
		{id: "doctor." + release.CodeAttestationMissing, code: release.CodeAttestationMissing},
		{id: "doctor." + release.CodeAttestationMismatch, code: release.CodeAttestationMismatch},
	}
}

// releaseMaterials are the generated releases a fixture names, made once
// per proof run under the test's temporary directory with the test
// identity: the good one and each tamper the entries use.
func releaseMaterials(t *testing.T) map[string]release.Inputs {
	t.Helper()
	good := releasetest.Generate(t, []byte("golaw doctor proof binary\n"), "1.2.3")
	out := map[string]release.Inputs{"good": good.Inputs(t)}
	for _, what := range []string{"signature", "identity", "attestation", "no-attestation"} {
		out[what] = good.Tamper(t, what).Inputs(t)
	}
	return out
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
	result := model.Result{Status: model.Measured(), Scope: c.id}
	if line := AssessRelease(f.Asset, release.Verify(c.inputs(f))); line.Code == c.code {
		result.Findings = append(result.Findings, model.Finding{
			Code: line.Code, Message: line.Detail, Severity: model.SeverityNote,
		})
	}
	return result, nil
}

// inputs are the files of the fixture, or the generated release it names.
func (c releaseDiagnosticCheck) inputs(f releaseFixture) release.Inputs {
	if f.Material == "" {
		return release.Inputs{
			BinaryDigest: f.BinaryDigest, Asset: f.Asset, Version: f.Version,
			Checksums: []byte(f.Checksums), Bundle: []byte(f.Bundle), Attestation: []byte(f.Attestation),
		}
	}
	in := c.materials[f.Material]
	if f.Trust == "none" {
		in.Trust = release.TrustRoot{}
	}
	return in
}

func registerReleaseProof(t *testing.T, builder *engine.Builder) {
	t.Helper()
	materials := releaseMaterials(t)
	for _, c := range releaseProofs() {
		c.materials = materials
		if err := builder.Register(c); err != nil {
			t.Fatal(err)
		}
	}
}
