package runner_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
)

// linkCase is one row of the symlink boundary: the target of the staged
// link and whether it leaves the tree.
type linkCase struct {
	name   string
	target string
	escape bool
}

func TestMaterializeSymlinkBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []linkCase{
		{"red-escaping-link", "../../outside.txt", true},
		{"near-contained-link", "target.txt", false},
		{"near-dangling-link", "missing.txt", false},
		{"near-cyclic-link", "link.txt", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := linkRepo(t, tc.target)
			parent := t.TempDir()
			outside := filepath.Join(parent, "outside.txt")
			if err := os.WriteFile(outside, []byte("untouched\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			probeErr := os.Symlink("target.txt", filepath.Join(t.TempDir(), "link"))
			tree, err := runner.Materialize(t.Context(), runner.MaterializeInput{
				Dir: repo.Root, Env: gittest.Isolated(),
				Parent: parent, Bounds: runner.DefaultBounds(),
			})
			if tc.escape || probeErr != nil {
				requireLinkLimitation(t, tree, err)
			} else {
				requireLinkKept(t, tree, err, tc.target)
			}
			requireOutsideUntouched(t, parent, outside)
		})
	}
}

// linkRepo is a repository whose index holds link.txt, a link to target.
func linkRepo(t *testing.T, target string) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	repo.Commit("chore: seed", map[string]string{"target.txt": "safe\n"})
	repo.Write("link-source.txt", target)
	blob := repo.Git("hash-object", "-w", "--", "link-source.txt")
	repo.Git("update-index", "--add", "--cacheinfo",
		"120000", blob, "link.txt")
	return repo
}

// requireLinkLimitation requires a link the tree cannot hold to be named
// as a limit, with no tree.
func requireLinkLimitation(t *testing.T, tree *runner.Tree, err error) {
	t.Helper()
	if tree != nil || !errors.Is(err, model.ErrCannotJudge) ||
		!strings.Contains(err.Error(), "link.txt") {
		t.Fatalf("link limitation = %v, %v", tree, err)
	}
}

// requireLinkKept requires a contained, dangling or cyclic link to keep
// its target in the tree, and the tree to close.
func requireLinkKept(t *testing.T, tree *runner.Tree, err error, want string) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tree.Dir, "link.txt")
	target, err := os.Readlink(link)
	if err != nil || target != want {
		t.Fatalf("link target = %q, %v", target, err)
	}
	requireLinkResolution(t, link, want)
	if closeErr := tree.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
}

// requireLinkResolution requires the contained link to read the bytes of
// its target, the dangling one to resolve to nothing and the cyclic one
// never to resolve.
func requireLinkResolution(t *testing.T, link, target string) {
	t.Helper()
	switch target {
	case "target.txt":
		data, err := os.ReadFile(link)
		if err != nil || string(data) != "safe\n" {
			t.Fatalf("link content = %q, %v", data, err)
		}
	case "missing.txt":
		_, err := os.Stat(link)
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("dangling target = %v", err)
		}
	case "link.txt":
		if _, err := os.Stat(link); err == nil {
			t.Fatal("cyclic target unexpectedly resolved")
		}
	}
}

// requireOutsideUntouched requires the file outside the tree to keep its
// bytes and the parent directory to hold nothing else.
func requireOutsideUntouched(t *testing.T, parent, outside string) {
	t.Helper()
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "untouched\n" {
		t.Fatalf("outside content = %q, %v", data, err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 1 ||
		entries[0].Name() != "outside.txt" {
		t.Fatalf("resources remain = %v, %v", entries, err)
	}
}
