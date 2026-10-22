package source_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

func TestFromCandidatesFoldsEveryDecodedOperation(t *testing.T) {
	t.Parallel()
	original := source.ContentHash([]byte("package a\n\nfunc A() {}\n"))
	rows := []struct {
		name          string
		candidate     model.Candidate
		content       string
		path, want    string
		gone          string
		reconstructed bool
	}{
		{"create", model.Candidate{Path: "new.go", Op: model.CandidateCreate, Reconstructed: true}, "package a\n", "new.go", "package a\n", "", true},
		{"edit applied", model.Candidate{Path: "a.go", Op: model.CandidateEdit, Reconstructed: true, BaseHash: original}, "package a\n\nfunc B() {}\n", "a.go", "package a\n\nfunc B() {}\n", "", true},
		{"edit unapplied", model.Candidate{Path: "docs/x.md", Op: model.CandidateEdit}, "added line", "docs/x.md", "added line", "", false},
		{"move applied", model.Candidate{Path: "c.go", Op: model.CandidateMove, From: "a.go", Reconstructed: true, BaseHash: original}, "package c\n", "c.go", "package c\n", "a.go", true},
		{"move unapplied", model.Candidate{Path: "d.go", Op: model.CandidateMove, From: "a.go"}, "added", "d.go", "added", "a.go", false},
		{"rename", model.Candidate{Path: "b.go", Op: model.CandidateMove, From: "a.go", Reconstructed: true, BaseHash: original}, "", "b.go", "package a\n\nfunc A() {}\n", "a.go", true},
		{"delete", model.Candidate{Path: "a.go", Op: model.CandidateDelete, Reconstructed: true, BaseHash: original}, "", "docs/x.md", "x\n", "a.go", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			foldOne(t, row.candidate, row.content, row.path, row.want, row.gone, row.reconstructed)
		})
	}
}

// foldOne folds one decoded candidate over the base and reads it back.
func foldOne(t *testing.T, candidate model.Candidate, text, path, want, gone string, reconstructed bool) {
	t.Helper()
	content := map[string][]byte{}
	if text != "" {
		content[candidate.Path] = []byte(text)
	}
	o, cands, err := source.NewOverlay(base(), source.FromCandidates([]model.Candidate{candidate}, content))
	if err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, o, path); got != want {
		t.Errorf("Read(%s) = %q, want %q", path, got, want)
	}
	if gone != "" && !errors.Is(readErr(o, gone), model.ErrNotInSnapshot) {
		t.Errorf("%s must leave the universe", gone)
	}
	if last := cands[len(cands)-1]; last.Reconstructed != reconstructed {
		t.Errorf("reconstructed = %v, want %v", last.Reconstructed, reconstructed)
	}
}
