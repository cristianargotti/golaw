package proof_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestStagedFixtureKeepsBaseBlobsSeparate(t *testing.T) {
	t.Parallel()
	fixture, err := proof.LoadFixture("testdata/policy.codeowners-foreign/red")
	if err != nil {
		t.Fatal(err)
	}
	input, err := fixture.Input(model.ScopeTree, proof.Policy())
	if err != nil {
		t.Fatal(err)
	}
	if input.Mode != model.ModeStaged || len(input.Changes) != 2 {
		t.Fatalf("input = %+v", input)
	}
	if input.Snapshot.Class(proof.StagedFile) != model.ClassUnknown {
		t.Fatal("staged metadata entered the measured tree")
	}
	base, err := model.ReadBlob(input.Snapshot, "base")
	if err != nil || string(base) != "* @app\n/.github/ @platform\n/.github/CODEOWNERS @app\n" {
		t.Fatalf("base = %q, error %v", base, err)
	}
	base[0] = 'x'
	again, err := model.ReadBlob(input.Snapshot, "base")
	if err != nil || again[0] != '*' {
		t.Fatalf("blob was mutable: %q, %v", again, err)
	}
	_, err = model.ReadBlob(input.Snapshot, "absent")
	if !errors.Is(err, model.ErrUnreadable) {
		t.Fatalf("missing blob error = %v", err)
	}
}

func TestBlobTreeKeepsReadBound(t *testing.T) {
	t.Parallel()
	tree := proof.NewBlobTree(map[string][]byte{}, map[string][]byte{
		"large": make([]byte, model.MaxReadBytes+1),
	})
	_, err := tree.Blob("large")
	if !errors.Is(err, model.ErrTooLarge) {
		t.Fatalf("bound error = %v", err)
	}
}
