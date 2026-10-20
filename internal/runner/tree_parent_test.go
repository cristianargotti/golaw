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

func TestTreeSweepAndMaterializeCreateMissingStateParent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{"file.txt": "recorded\n"})
	state := t.TempDir()
	parent := filepath.Join(state, "state", "runner", "trees")
	if err := runner.Sweep(parent); err != nil {
		t.Fatalf("sweep before first use: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "state")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("sweep created resources: %v", err)
	}
	tree, err := runner.Materialize(t.Context(), runner.MaterializeInput{
		Dir: repo.Root, Env: gittest.Isolated(), Parent: parent,
		Bounds: runner.DefaultBounds(),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(state, "state"), filepath.Dir(parent), parent, tree.Dir} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			t.Errorf("directory is not private: %s mode=%v", path, info.Mode())
		}
	}
	data, readErr := os.ReadFile(filepath.Join(tree.Dir, "file.txt"))
	if readErr != nil || string(data) != "recorded\n" {
		t.Errorf("materialized bytes = %q, %v", data, readErr)
	}
	if closeErr := tree.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	entries, readErr := os.ReadDir(parent)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("resources remain = %v, %v", entries, readErr)
	}
}
