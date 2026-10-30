package policyfiles

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type buildSelectingSnapshot struct {
	*proof.Tree

	filesCalls  int
	selectCalls int
	anyCalls    int
}

func (s *buildSelectingSnapshot) Files() []model.File {
	s.filesCalls++
	return s.Tree.Files()
}

func (s *buildSelectingSnapshot) Select(match func(string) bool) []model.File {
	s.selectCalls++
	return model.Select(s.Tree, match)
}

func (s *buildSelectingSnapshot) Any(match func(string) bool) bool {
	s.anyCalls++
	return model.Any(s.Tree, match)
}

func TestMakefileGlobKeepsSelectiveSnapshotReads(t *testing.T) {
	t.Parallel()
	in := makeInput(nil)
	snapshot := &buildSelectingSnapshot{Tree: proof.NewTree(map[string][]byte{
		"Makefile":     []byte("include mk/*.mk\n"),
		"mk/checks.mk": []byte("scan lint-surface:\n"),
	})}
	in.Snapshot = snapshot
	got, err := Makefile().Run(t.Context(), in)
	if err != nil || got.Status.Kind != model.StatusMeasured || len(got.Findings) != 0 {
		t.Fatalf("glob result = %+v, %v", got, err)
	}
	if snapshot.selectCalls != 1 || snapshot.filesCalls != 0 {
		t.Fatalf("glob selection called Select %d times and Files %d times", snapshot.selectCalls, snapshot.filesCalls)
	}
}

func TestBuildSnapshotKeepsSelectiveExistenceReads(t *testing.T) {
	t.Parallel()
	snapshot := &buildSelectingSnapshot{Tree: proof.NewTree(map[string][]byte{"Makefile": nil})}
	wrapped := &buildSnapshot{Snapshot: snapshot}
	if !model.Any(wrapped, func(path string) bool { return path == "Makefile" }) {
		t.Fatal("wrapped snapshot lost an existing path")
	}
	if snapshot.anyCalls != 1 || snapshot.filesCalls != 0 {
		t.Fatalf("existence selection called Any %d times and Files %d times", snapshot.anyCalls, snapshot.filesCalls)
	}
}
