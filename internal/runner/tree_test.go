package runner_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

func TestTreeChild(t *testing.T) {
	t.Parallel()
	dir, enabled := os.LookupEnv("GOLAW_TREE_REPO")
	if !enabled {
		return
	}
	tree, err := runner.Materialize(t.Context(), runner.MaterializeInput{
		Dir: dir, Env: gittest.Isolated(),
		Parent: os.Getenv("GOLAW_TREE_PARENT"), Bounds: runner.DefaultBounds(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		os.Getenv("GOLAW_READY"), []byte(tree.Dir+"\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Second)
	if err := tree.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTreeSweepRespectsOwnership(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{"keep.txt": "keep\n"})
	parent := t.TempDir()
	foreign := filepath.Join(parent, "foreign")
	if err := os.Mkdir(foreign, 0o700); err != nil {
		t.Fatal(err)
	}
	cmd, tree := startRunnerChild(t, "TestTreeChild", []string{
		"GOLAW_TREE_REPO=" + repo.Root,
		"GOLAW_TREE_PARENT=" + parent,
	})
	for _, tc := range []struct {
		name string
		kill bool
	}{
		{"near-live-owner", false},
		{"red-ended-owner", true},
	} {
		if tc.kill {
			stopRunnerChild(t, cmd)
		}
		requireSweep(t, tc.name, tc.kill, parent, tree, foreign)
	}
	if err := runner.Sweep(parent); err != nil {
		t.Fatal(err)
	}
	err := (&runner.Tree{Dir: foreign}).Close()
	if err == nil {
		t.Fatal("foreign directory accepted as an owned tree")
	}
	if _, statErr := os.Stat(foreign); statErr != nil {
		t.Fatalf("foreign directory changed: %v", statErr)
	}
	if got := runner.DefaultBounds(); got != (runner.Bounds{
		Bytes: 512 << 20, Files: 50000,
	}) {
		t.Fatalf("default bounds = %+v", got)
	}
}

// requireSweep requires a sweep to remove only the tree of an ended owner,
// keeping the tree of a live owner and a foreign directory.
func requireSweep(t *testing.T, name string, ended bool, parent, tree, foreign string) {
	t.Helper()
	if err := runner.Sweep(parent); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	_, err := os.Stat(tree)
	if ended && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s: tree remains: %v", name, err)
	}
	if !ended && err != nil {
		t.Fatalf("%s: live tree removed: %v", name, err)
	}
	if _, statErr := os.Stat(foreign); statErr != nil {
		t.Fatalf("%s: foreign directory removed: %v", name, statErr)
	}
}
