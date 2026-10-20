package runner_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

// TestMaterializeLeavesOutIntentsToAdd pins that a copy of the index holds
// what the next commit records: git add -N puts the empty blob in the index
// for a path the commit never writes, so a copy that kept it would hand the
// Go steps an empty file, while a file staged empty stays.
func TestMaterializeLeavesOutIntentsToAdd(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		seed   bool
		remove bool
	}{
		{"red-intent", true, false},
		{"red-intent-file-removed", true, true},
		{"red-intent-unborn-branch", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			if tc.seed {
				repo.Commit("chore: seed", map[string]string{"kept.go": "package kept\n"})
			}
			repo.Write("intent.go", "package kept\n\nconst Intent = 1\n")
			repo.Git("add", "-N", "--", "intent.go")
			repo.Stage(map[string]string{"empty.txt": ""})
			if tc.remove {
				repo.Remove("intent.go")
			}
			tree, err := runner.Materialize(t.Context(), runner.MaterializeInput{
				Dir: repo.Root, Env: gittest.Isolated(),
				Parent: t.TempDir(), Bounds: runner.DefaultBounds(),
			})
			if err != nil {
				t.Fatal(err)
			}
			requireIntentLeftOut(t, tree, tc.seed)
		})
	}
}

// requireIntentLeftOut requires the copy to lack the intent, to hold the
// file staged empty as an empty file and the committed file as committed,
// and to close.
func requireIntentLeftOut(t *testing.T, tree *runner.Tree, seeded bool) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(tree.Dir, "intent.go")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("intent to add materialized: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tree.Dir, "empty.txt"))
	if err != nil || len(data) != 0 {
		t.Errorf("empty staged file = %q, %v", data, err)
	}
	if seeded {
		kept, keptErr := os.ReadFile(filepath.Join(tree.Dir, "kept.go"))
		if keptErr != nil || string(kept) != "package kept\n" {
			t.Errorf("committed file = %q, %v", kept, keptErr)
		}
	}
	if closeErr := tree.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
}

// TestMaterializeKeepsAColonOutsideWindows pins that a name holding a
// colon, such as a timestamp, is copied where the host takes it.
func TestMaterializeKeepsAColonOutsideWindows(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows reads a colon as a drive or a stream")
	}
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{"2026-10-20T10:00:00Z.json": "{}\n"})
	tree, err := runner.Materialize(t.Context(), runner.MaterializeInput{
		Dir: repo.Root, Env: gittest.Isolated(),
		Parent: t.TempDir(), Bounds: runner.DefaultBounds(),
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tree.Dir, "2026-10-20T10:00:00Z.json"))
	if err != nil || string(data) != "{}\n" {
		t.Errorf("colon name = %q, %v", data, err)
	}
	if closeErr := tree.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
}
