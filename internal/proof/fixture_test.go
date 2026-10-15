package proof_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// emDash is built from its code point so this file carries none.
const emDash = rune(0x2014)

func TestUnescape(t *testing.T) {
	t.Parallel()
	dash := string(emDash)
	cases := map[string]string{
		"a " + escaped + " b": "a " + dash + " b",
		"back\\\\slash":       "back\\slash",
		"tail\\":              "tail\\",
		"\\x41":               "\\x41",
		"\\u12":               "\\u12",
		"\\uZZZZ":             "\\uZZZZ",
		"plain":               "plain",
	}
	for in, want := range cases {
		if got := string(proof.Unescape([]byte(in))); got != want {
			t.Errorf("Unescape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoadFixture(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	red, err := proof.LoadFixture(filepath.Join(root, "testdata", "text.dashes", "red"))
	if err != nil {
		t.Fatal(err)
	}
	if string(red.Files["docs/a.md"]) != "title "+string(emDash)+" subtitle\n" || red.Event != nil {
		t.Errorf("red = %+v", red)
	}
	event, err := proof.LoadFixture(filepath.Join(root, "testdata", "shell.no-verify", "red"))
	if err != nil {
		t.Fatal(err)
	}
	if event.Event == nil || event.Event.Shape != model.ShapeShell || len(event.Files) != 0 {
		t.Errorf("event fixture = %+v", event)
	}
}

func TestLoadFixtureRefusesBrokenTrees(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "bad-json/event.json", "{")
	write(t, root, "bad-event/event.json", `{"shape":"read","agent":"tester","tool":"Read"}`)
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bad-json", "bad-event", "empty", "absent"} {
		if _, err := proof.LoadFixture(filepath.Join(root, name)); !errors.Is(err, proof.ErrFixture) {
			t.Errorf("%s: %v, want ErrFixture", name, err)
		}
	}
}

func TestFixtureInput(t *testing.T) {
	t.Parallel()
	files := proof.Fixture{Files: map[string][]byte{"a.go": []byte("package a\n"), "b.go": []byte("package b\n")}}
	modes := map[model.Scope]model.Mode{model.ScopeFile: model.ModeScan, model.ScopeTree: model.ModeScan, model.ScopeCommit: model.ModeCommit}
	for scope, mode := range modes {
		in, err := files.Input(scope, proof.Policy())
		if err != nil || in.Mode != mode || len(in.Targets) != 2 || in.Event != nil || in.Policy == nil || in.Snapshot == nil {
			t.Errorf("%s: %+v, %v", scope, in, err)
		}
	}
	if _, err := files.Input(model.ScopeEvent, proof.Policy()); !errors.Is(err, proof.ErrFixture) {
		t.Errorf("event scope without event: %v", err)
	}
	if _, err := files.Input(0, proof.Policy()); !errors.Is(err, proof.ErrFixture) {
		t.Errorf("invalid scope: %v", err)
	}
	candidates := []model.Candidate{{Path: "a.go", Op: model.CandidateReplace, Reconstructed: true}, {Path: "gone.go", Op: model.CandidateDelete}}
	writeEvent := proof.Fixture{Files: files.Files, Event: &model.Event{Shape: model.ShapeWrite, Agent: "tester", Tool: "Write", Candidates: candidates}}
	in, err := writeEvent.Input(model.ScopeEvent, proof.Policy())
	if err != nil || in.Mode != model.ModeHook || len(in.Targets) != 1 || in.Targets[0].Path != "a.go" || in.Targets[0].Size != 10 {
		t.Errorf("write event: %+v, %v", in.Targets, err)
	}
	post := proof.Fixture{Files: files.Files, Event: &model.Event{Shape: model.ShapePost, Agent: "tester", Tool: "Bash", Post: &model.PostResult{Changed: []string{"b.go"}}}}
	if in, err := post.Input(model.ScopeEvent, proof.Policy()); err != nil || len(in.Targets) != 1 || in.Targets[0].Path != "b.go" {
		t.Errorf("post event: %+v, %v", in.Targets, err)
	}
	read := proof.Fixture{Event: &model.Event{Shape: model.ShapeRead, Agent: "tester", Tool: "Read", Read: &model.ReadRequest{Path: "x"}}}
	if in, err := read.Input(model.ScopeEvent, proof.Policy()); err != nil || len(in.Targets) != 0 {
		t.Errorf("read event: %+v, %v", in.Targets, err)
	}
}
