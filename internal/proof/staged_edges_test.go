package proof_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestStagedFixtureRejectsInvalidMetadata(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"syntax":           `{`,
		"unknown field":    `{"changed":[]}`,
		"unknown change":   `{"changes":[{"Op":1,"NewPath":"a","extra":true}]}`,
		"trailing object":  `{} {}`,
		"trailing garbage": `{} invalid`,
		"null":             `null`,
		"zero operation":   `{"changes":[{"NewPath":"a"}]}`,
		"unknown op":       `{"changes":[{"Op":5,"NewPath":"a"}]}`,
		"add old path":     `{"changes":[{"Op":1,"OldPath":"b","NewPath":"a"}]}`,
		"add old blob":     `{"changes":[{"Op":1,"Before":"base","NewPath":"a"}]}`,
		"add no path":      `{"changes":[{"Op":1}]}`,
		"modify no path":   `{"changes":[{"Op":2}]}`,
		"modify new path":  `{"changes":[{"Op":2,"OldPath":"a","NewPath":"b"}]}`,
		"delete new path":  `{"changes":[{"Op":3,"OldPath":"a","NewPath":"a"}]}`,
		"delete new blob":  `{"changes":[{"Op":3,"OldPath":"a","After":"candidate"}]}`,
		"delete no path":   `{"changes":[{"Op":3}]}`,
		"rename old path":  `{"changes":[{"Op":4,"OldPath":"a"}]}`,
		"rename new path":  `{"changes":[{"Op":4,"NewPath":"a"}]}`,
		"rename same path": `{"changes":[{"Op":4,"OldPath":"a","NewPath":"a"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, "case/"+proof.StagedFile, content)
			write(t, root, "case/a", "candidate\n")
			_, err := proof.LoadFixture(filepath.Join(root, "case"))
			if !errors.Is(err, proof.ErrFixture) {
				t.Fatalf("LoadFixture() error = %v, want ErrFixture", err)
			}
			if strings.Count(err.Error(), proof.ErrFixture.Error()) != 1 {
				t.Fatalf("fixture error repeated: %v", err)
			}
		})
	}
}

func TestStagedFixtureNamesContradictoryChange(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		operation, name string
	}{
		{"1", "add"},
		{"5", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			write(t, root, "case/"+proof.StagedFile, `{"changes":[{"Op":1,"NewPath":"a"},{"Op":`+tc.operation+`}]}`)
			_, err := proof.LoadFixture(filepath.Join(root, "case"))
			if !errors.Is(err, proof.ErrFixture) {
				t.Fatalf("LoadFixture() error = %v, want ErrFixture", err)
			}
			want := "contradictory staged change at index 1 (operation " + tc.name + ", " + tc.operation + ")"
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not contain %q", err, want)
			}
		})
	}
}

func TestStagedFixtureAcceptsEveryOperation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "case/"+proof.StagedFile, `{"changes":[
		{"Op":1,"NewPath":"added"},
		{"Op":2,"OldPath":"modified","NewPath":"modified"},
		{"Op":3,"OldPath":"deleted"},
		{"Op":4,"OldPath":"before","NewPath":"after"}
	]}`+"\n")
	fixture, err := proof.LoadFixture(filepath.Join(root, "case"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.Files) != 0 || fixture.Staged == nil || len(fixture.Staged.Changes) != 4 {
		t.Fatalf("fixture = %+v", fixture)
	}
	for _, scope := range []model.Scope{model.ScopeFile, model.ScopeTree} {
		input, err := fixture.Input(scope, proof.Policy())
		if err != nil || input.Mode != model.ModeStaged || len(input.Changes) != 4 || len(input.Targets) != 0 {
			t.Fatalf("%s input = %+v, error %v", scope, input, err)
		}
	}
}

func TestStagedFixtureInputCopiesEvidence(t *testing.T) {
	t.Parallel()
	fixture := proof.Fixture{
		Files: map[string][]byte{"candidate": []byte("candidate\n")},
		Staged: &proof.StagedFixture{
			Changes: []model.Change{{Op: model.ChangeDelete, OldPath: "deleted", Before: "opaque blob"}},
			Blobs:   map[string]string{"opaque blob": "base\n"},
		},
	}
	input, err := fixture.Input(model.ScopeTree, proof.Policy())
	if err != nil {
		t.Fatal(err)
	}
	fixture.Files["candidate"][0] = 'x'
	fixture.Staged.Changes[0].OldPath = "changed"
	fixture.Staged.Blobs["opaque blob"] = "changed\n"
	data, err := input.Snapshot.Read("candidate")
	if err != nil || string(data) != "candidate\n" {
		t.Fatalf("candidate = %q, error %v", data, err)
	}
	data, err = model.ReadBlob(input.Snapshot, "opaque blob")
	if err != nil || string(data) != "base\n" || input.Changes[0].OldPath != "deleted" {
		t.Fatalf("base = %q, changes = %+v, error %v", data, input.Changes, err)
	}
	input.Changes[0].OldPath = "input edit"
	if fixture.Staged.Changes[0].OldPath != "changed" {
		t.Fatal("input changes mutated the fixture")
	}
}

func TestBlobTreeCopiesSourcesAndKeepsOpaqueIDs(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{"candidate": []byte("candidate\n")}
	blobs := map[string][]byte{"a path/id": []byte("base\n"), "empty": nil}
	tree := proof.NewBlobTree(files, blobs)
	files["candidate"][0], blobs["a path/id"][0] = 'x', 'x'
	delete(blobs, "empty")
	data, err := tree.Read("candidate")
	if err != nil || string(data) != "candidate\n" {
		t.Fatalf("candidate = %q, error %v", data, err)
	}
	data, err = tree.Blob("a path/id")
	if err != nil || string(data) != "base\n" {
		t.Fatalf("base = %q, error %v", data, err)
	}
	if data, err := tree.Blob("empty"); err != nil || len(data) != 0 {
		t.Fatalf("empty = %q, error %v", data, err)
	}
	if tree.Class("a path/id") != model.ClassUnknown || len(tree.Files()) != 1 {
		t.Fatal("base blobs entered the candidate tree")
	}
}

func TestStagedFixtureLeavesCommitAndEventModesUnchanged(t *testing.T) {
	t.Parallel()
	fixture := proof.Fixture{
		Files:  map[string][]byte{"a": []byte("content\n")},
		Staged: &proof.StagedFixture{Changes: []model.Change{{Op: model.ChangeDelete, OldPath: "gone"}}},
		Commit: &model.Commit{Message: "fix: retain commit mode\n"},
		Event: &model.Event{
			Shape: model.ShapeRead, Agent: "tester", Tool: "Read",
			Read: &model.ReadRequest{Path: "a"},
		},
	}
	for scope, mode := range map[model.Scope]model.Mode{
		model.ScopeCommit: model.ModeCommit,
		model.ScopeEvent:  model.ModeHook,
	} {
		input, err := fixture.Input(scope, proof.Policy())
		if err != nil || input.Mode != mode {
			t.Fatalf("%s input = %+v, error %v", scope, input, err)
		}
		if _, err := model.ReadBlob(input.Snapshot, "anything"); !errors.Is(err, model.ErrUnreadable) {
			t.Fatalf("%s unexpectedly provides staged blobs: %v", scope, err)
		}
	}
}
