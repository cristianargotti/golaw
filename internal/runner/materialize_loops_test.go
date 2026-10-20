package runner_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

// TestMaterializeBoundsTheLinksOneTargetFollows pins that links which name
// each other twice per target cannot make the validation of a copy grow
// without end: no host follows more than 63 links for one path, so a target
// that needs more is kept, as the loop the host sees, and the copy is made
// in milliseconds where an unbounded walk would take hours.
func TestMaterializeBoundsTheLinksOneTargetFollows(t *testing.T) {
	t.Parallel()
	if err := os.Symlink("target", filepath.Join(t.TempDir(), "link")); err != nil {
		t.Skipf("host cannot create a symlink: %v", err)
	}
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{"keep.txt": "keep\n"})
	links := map[string]string{"l00": "."}
	for i := 1; i <= 24; i++ {
		previous := fmt.Sprintf("l%02d", i-1)
		links[fmt.Sprintf("l%02d", i)] = previous + "/" + previous
	}
	for name, target := range links {
		repo.Write("source-"+name, target)
		blob := repo.Git("hash-object", "-w", "--", "source-"+name)
		repo.Git("update-index", "--add", "--cacheinfo", "120000", blob, name)
	}
	start := time.Now()
	tree, err := runner.Materialize(t.Context(), runner.MaterializeInput{
		Dir: repo.Root, Env: gittest.Isolated(),
		Parent: t.TempDir(), Bounds: runner.DefaultBounds(),
	})
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("materialization took %s", took)
	}
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range links {
		if got, readErr := os.Readlink(filepath.Join(tree.Dir, name)); readErr != nil || got != want {
			t.Errorf("link %s = %q, %v", name, got, readErr)
		}
	}
	if closeErr := tree.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
}
