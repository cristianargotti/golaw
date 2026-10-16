package cicheck_test

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestAuditProof(t *testing.T) {
	t.Parallel()
	manifest, err := proof.LoadManifest("proof/manifest.yml")
	if err != nil {
		t.Fatal(err)
	}
	registry := registryOf(t, manifest, proofRules(t))
	for _, failure := range proof.Verify(t.Context(), registry, "proof", manifest) {
		t.Error(failure.String())
	}
}

func TestRepositoryAssets(t *testing.T) {
	t.Parallel()
	manifest, err := proof.LoadManifest("proof/manifest.yml")
	if err != nil {
		t.Fatal(err)
	}
	tree := kitTree(t)
	run, err := engine.Dispatch(t.Context(), auditRegistry(t, manifest), model.Input{
		Mode: model.ModeScan, Policy: proof.Policy(),
		Snapshot: tree, Targets: tree.Files(),
	})
	if err != nil {
		t.Fatal(err)
	}
	decision := engine.Decide(run)
	if decision.Verdict != engine.VerdictClean {
		t.Fatalf("repository asset audit failed: %+v", decision)
	}
}

func TestPublicTextBoundaries(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte("acme"))
	inspect := newPublicTextInspector([]string{hex.EncodeToString(sum[:])})
	for _, text := range []string{
		"heading\u2013detail",
		"heading\u2014detail",
		"acme",
		strings.ToUpper("acme"),
		"before/acme.after",
	} {
		if inspect("docs/nested/a.md", []byte(text)) == "" {
			t.Errorf("prohibited text accepted: %q", text)
		}
	}
	if got := inspect("README.md", []byte("A neutral team - acmes Go rules.")); got != "" {
		t.Fatal(got)
	}
	for _, path := range []string{
		"README.md", "CONTRIBUTING.md", "docs/adr/0001.md",
		"agents/SKILL.md", "docs/reference.txt", "docs/reference.rst",
	} {
		if !documentation(path) {
			t.Errorf("documentation excluded: %s", path)
		}
	}
	if documentation("internal/example.go") {
		t.Fatal("source classified as documentation")
	}
	for _, path := range []string{"internal/example.go", "golaw.yml", "agents/manifest.json"} {
		if !publicTextRule().match(path) {
			t.Errorf("the private name is not read in %s", path)
		}
	}
}
