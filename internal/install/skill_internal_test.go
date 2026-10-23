package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestSkillPath(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	root := filepath.Join(home, "repository")
	for _, row := range []struct {
		scope string
		want  string
	}{
		{"", filepath.Join(home, ".agents", "skills", "golaw", "SKILL.md")},
		{"user", filepath.Join(home, ".agents", "skills", "golaw", "SKILL.md")},
		{"project", filepath.Join(root, ".agents", "skills", "golaw", "SKILL.md")},
	} {
		got := SkillPath(Options{Home: home, Root: root, Scope: row.scope})
		if got != row.want {
			t.Errorf("scope %q: path = %q, want %q", row.scope, got, row.want)
		}
	}
}

func TestSkillOwners(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, scope, otherScope, otherIdentity, otherPath string
		want                                              []string
	}{
		{"shared user", "user", "user", "project", "same", []string{"cursor"}},
		{"shared project", "project", "project", "project", "same", []string{"cursor"}},
		{"other scope", "user", "project", "project", "same", nil},
		{"other project", "project", "project", "other", "same", nil},
		{"other user identity", "user", "user", "other", "same", []string{"cursor"}},
		{"other path", "user", "user", "project", "other", nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			home := cinHome(t)
			state := filepath.Join(home, "state")
			path := filepath.Join(home, ".agents", "skills", "golaw", "SKILL.md")
			otherPath := path
			if row.otherPath != "same" {
				otherPath = filepath.Join(home, "other", "SKILL.md")
			}
			cinSkillReceipt(t, state, "codex", row.scope, "project", path)
			cinSkillReceipt(t, state, "cursor", row.otherScope, row.otherIdentity, otherPath)
			got, err := SkillOwners(state, row.scope, "project", "codex", path)
			if err != nil || !slices.Equal(got, row.want) {
				t.Fatalf("owners = %v, %v; want %v", got, err, row.want)
			}
		})
	}
}

func TestSkillOwnersMissingAndCorrupt(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	state := filepath.Join(home, "state")
	path := filepath.Join(home, ".agents", "skills", "golaw", "SKILL.md")
	got, err := SkillOwners(state, "user", "project", "codex", path)
	if err != nil || len(got) != 0 {
		t.Fatalf("absent state = %v, %v", got, err)
	}
	cinWrite(t, filepath.Join(state, "cursor", "user", "receipt.json"), []byte("{"))
	if _, readErr := SkillOwners(state, "user", "project", "codex", path); readErr == nil {
		t.Fatal("corrupt ownership evidence accepted")
	}
}

func cinSkillReceipt(t *testing.T, state, agent, scope, identity, path string) {
	t.Helper()
	dir := filepath.Join(state, agent, scope)
	if scope == "project" {
		dir = filepath.Join(dir, identity)
	}
	receipt := Receipt{
		Schema: 1, ProjectID: identity, Scope: scope, Version: "0.4.0",
		Agent: agent, Bundle: dir,
		Artifacts: []Artifact{{Path: path, SHA256: Digest([]byte("skill"))}},
	}
	data, err := json.Marshal(receipt)
	must(t, err)
	cinWrite(t, filepath.Join(dir, "receipt.json"), data)
}

// TestSkillOwnersReadsOnlyPlainReceipts holds the reader to the receipt
// each agent directory keeps: a file beside the agent directories is not
// evidence, while a link on the way to a receipt, a state that is no
// directory, a scope path that is no directory and a project without its
// identity are refused instead of read; where the system reads a file in
// the place of a directory as nothing there, those two hold no receipt.
func TestSkillOwnersReadsOnlyPlainReceipts(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	state := filepath.Join(home, "state")
	path := filepath.Join(home, ".agents", "skills", "golaw", "SKILL.md")
	cinSkillReceipt(t, state, "cursor", "user", "project", path)
	cinWrite(t, filepath.Join(state, "notes"), []byte("not an agent"))
	got, err := SkillOwners(state, "user", "project", "codex", path)
	if err != nil || !slices.Equal(got, []string{"cursor"}) {
		t.Fatalf("owners beside a stray file = %v, %v", got, err)
	}
	if _, err := SkillOwners(state, "project", "", "codex", path); !errors.Is(err, ErrPath) {
		t.Fatalf("project without identity = %v", err)
	}
	switch got, err := SkillOwners(filepath.Join(state, "notes"), "user", "project", "codex", path); {
	case notDirIsAbsent && (err != nil || len(got) != 0):
		t.Fatalf("a state that is a file, which this system reads as absent = %v, %v", got, err)
	case !notDirIsAbsent && err == nil:
		t.Fatal("a state that is a file was read")
	}
	cinWrite(t, filepath.Join(state, "other", "user"), []byte("not a directory"))
	switch got, err := SkillOwners(state, "user", "project", "codex", path); {
	case notDirIsAbsent && (err != nil || !slices.Equal(got, []string{"cursor"})):
		t.Fatalf("a scope path that is a file, which this system reads as absent = %v, %v", got, err)
	case !notDirIsAbsent && err == nil:
		t.Fatal("a scope path that is a file was accepted")
	}
	linked := filepath.Join(home, "linked")
	cinSkillReceipt(t, linked, "elsewhere", "user", "project", path)
	if err := os.Remove(filepath.Join(state, "other", "user")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(linked, "elsewhere", "user"), filepath.Join(state, "other", "user")); err != nil {
		t.Skipf("links are not available here: %v", err)
	}
	if _, err := SkillOwners(state, "user", "project", "codex", path); !errors.Is(err, ErrPath) {
		t.Fatalf("a linked receipt directory = %v", err)
	}
}
