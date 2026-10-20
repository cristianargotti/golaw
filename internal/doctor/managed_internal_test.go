package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
)

func TestManagedLinesNameEveryRecordedFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for name, content := range map[string]string{
		"lefthook.yml": "recorded\n",
		"golaw.mk":     "edited by hand\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	recorded := install.Digest([]byte("recorded\n"))
	facts := CollectManaged(root, map[string]string{
		"lefthook.yml": recorded, "golaw.mk": recorded,
		"tools/go.mod": recorded, "scripts": recorded,
	})
	encoded, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	var replay []ManagedFact
	if err := json.Unmarshal(encoded, &replay); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(AssessManaged(facts), AssessManaged(replay)) {
		t.Fatal("serialized evidence changed the assessment")
	}
	lines := AssessManaged(facts)
	got := make([]string, 0, len(lines))
	for _, line := range lines {
		got = append(got, line.Key+" "+line.State+" "+line.Code)
	}
	want := []string{
		"golaw.mk drift managed-drift",
		"lefthook.yml ok managed-matches",
		"scripts unknown managed-unreadable",
		"tools/go.mod missing managed-missing",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("managed lines = %v, want %v", got, want)
	}
	drift := lines[0]
	if drift.Detail != "golaw.mk differs from the hash init recorded: golaw init --update shows the diff and keeps the edit" {
		t.Fatalf("drift detail = %q", drift.Detail)
	}
	missing := lines[3]
	if missing.Detail != "tools/go.mod is gone from the checkout, and golaw init --update keeps a deletion as a local edit: restore it with git restore --source=HEAD --staged --worktree -- tools/go.mod" {
		t.Fatalf("missing detail = %q", missing.Detail)
	}
	if lines := AssessManaged(CollectManaged(root, nil)); len(lines) != 0 {
		t.Fatalf("a policy that manages no file = %+v", lines)
	}
}
