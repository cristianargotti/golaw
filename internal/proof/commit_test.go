package proof_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestLoadFixtureReadsTheCommitOfACommitCheck(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "shape/"+proof.CommitFile, `{"message":"fix: split the reader\n\n# a comment line\n","cleanup":"strip","author":"tester <tester@example.com>"}`)
	write(t, root, "shape/internal/a.go", "package a\n")
	write(t, root, "message/"+proof.CommitFile, `{"message":"docs: say why \u2014 always\n"}`)
	shape, err := proof.LoadFixture(filepath.Join(root, "shape"))
	if err != nil {
		t.Fatal(err)
	}
	if shape.Commit == nil || shape.Commit.Cleanup != model.CleanupStrip || shape.Commit.Author != "tester <tester@example.com>" {
		t.Errorf("commit = %+v", shape.Commit)
	}
	if _, listed := shape.Files[proof.CommitFile]; listed || len(shape.Files) != 1 || shape.Event != nil {
		t.Errorf("files = %v, the commit is not a file of the tree", shape.Files)
	}
	message, err := proof.LoadFixture(filepath.Join(root, "message"))
	if err != nil {
		t.Fatal(err)
	}
	if message.Commit == nil || !strings.ContainsRune(message.Commit.Message, emDash) || len(message.Files) != 0 {
		t.Errorf("a commit alone is a fixture whose message JSON escapes: %+v", message)
	}
}

func TestLoadFixtureRefusesABrokenCommit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "bad-json/"+proof.CommitFile, "{")
	write(t, root, "unknown-key/"+proof.CommitFile, `{"message":"x","body":"y"}`)
	write(t, root, "unresolved/"+proof.CommitFile, `{"message":"x","cleanup":"default"}`)
	write(t, root, "trailing-garbage/"+proof.CommitFile, `{"message":"fix: x"} garbage`)
	write(t, root, "second-object/"+proof.CommitFile, `{"message":"fix: x"}`+"\n"+`{"message":"fix: y"}`)
	for _, name := range []string{"bad-json", "unknown-key", "unresolved", "trailing-garbage", "second-object"} {
		if _, err := proof.LoadFixture(filepath.Join(root, name)); !errors.Is(err, proof.ErrFixture) {
			t.Errorf("%s: %v, want ErrFixture", name, err)
		}
	}
	write(t, root, "line-break/"+proof.CommitFile, `{"message":"fix: x"}`+"\n")
	if _, err := proof.LoadFixture(filepath.Join(root, "line-break")); err != nil {
		t.Errorf("a line break after the commit is no data after it: %v", err)
	}
}

func TestCommitFixtureInputIsACommitThatAddsItsFiles(t *testing.T) {
	t.Parallel()
	commit := &model.Commit{Message: "feat: add a and b\n"}
	files := map[string][]byte{"b.go": []byte("package b\n"), "a.go": []byte("package a\n")}
	fixture := proof.Fixture{Files: files, Commit: commit}
	in, err := fixture.Input(model.ScopeCommit, proof.Policy())
	if err != nil {
		t.Fatal(err)
	}
	if in.Mode != model.ModeCommit || in.Commit != commit || len(in.Targets) != 2 || in.Event != nil {
		t.Errorf("input = %+v", in)
	}
	sum := sha256.Sum256([]byte("package a\n"))
	want := model.Change{Op: model.ChangeAdd, NewPath: "a.go", Mode: "100644", After: hex.EncodeToString(sum[:])}
	if len(in.Changes) != 2 || in.Changes[0] != want || in.Changes[1].NewPath != "b.go" {
		t.Errorf("changes = %+v, want %+v first", in.Changes, want)
	}
	if _, err := (&proof.Fixture{Files: files}).Input(model.ScopeCommit, proof.Policy()); !errors.Is(err, proof.ErrFixture) {
		t.Errorf("commit scope without a commit: %v, want ErrFixture", err)
	}
}
