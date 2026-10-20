package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestTreeOwnerRefusesMissingMetadata(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	tree, err := newTree(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(filepath.Dir(tree.Dir), treeOwner)); err != nil {
		t.Fatal(err)
	}
	if err := tree.Close(); err == nil {
		t.Fatal("close accepted an envelope without ownership metadata")
	}
	if _, err := os.Stat(tree.Dir); err != nil {
		t.Fatalf("unverified envelope changed: %v", err)
	}
}

func TestTreeParentRejectsRepositories(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"worktree", "linked-worktree", "bare", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			repo := t.TempDir()
			setupTreeRepository(t, repo, kind)
			parent := filepath.Join(repo, "pending", "trees")
			if kind == "symlink" {
				alias := filepath.Join(t.TempDir(), "alias")
				if err := os.Symlink(repo, alias); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				parent = filepath.Join(alias, "pending", "trees")
			}
			// A copy inside a repository is a place the kit cannot judge in,
			// so a rung leaves its Go steps to the pipeline instead of failing.
			if tree, err := newTree(parent); !errors.Is(err, model.ErrCannotJudge) || tree != nil {
				t.Fatalf("repository parent accepted: %v, %v", tree, err)
			}
			if _, err := os.Stat(filepath.Join(repo, "pending")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("repository changed: %v", err)
			}
		})
	}
}

func setupTreeRepository(t *testing.T, path, kind string) {
	t.Helper()
	if kind == "bare" {
		for _, name := range []string{"objects", "refs"} {
			if err := os.Mkdir(filepath.Join(path, name), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(path, "HEAD"), []byte("ref: refs/heads/main\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	if kind == "linked-worktree" {
		if err := os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: elsewhere\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.Mkdir(filepath.Join(path, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestTreeCloseRejectsSubstitutedEnvelope(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	tree, err := newTree(parent)
	if err != nil {
		t.Fatal(err)
	}
	envelope := filepath.Dir(tree.Dir)
	retained := envelope + ".retained"
	if err := os.Rename(envelope, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(retained, envelope); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := tree.Close(); err == nil {
		t.Fatal("close followed a substituted envelope")
	}
	if _, err := os.Stat(filepath.Join(retained, treePayload)); err != nil {
		t.Fatalf("link target changed: %v", err)
	}
}

func TestTreeCleanupDoesNotFollowPayloadLinks(t *testing.T) {
	t.Parallel()
	tree, err := newTree(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	keep := filepath.Join(outside, "keep")
	if err := os.WriteFile(keep, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(tree.Dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, tree.Dir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := tree.Close(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(keep); err != nil || string(data) != "safe" {
		t.Fatalf("outside content = %q, %v", data, err)
	}
}

func TestTreeCreatorNames(t *testing.T) {
	t.Parallel()
	valid := fmt.Sprintf("%s%d-%s", treePrefix, os.Getpid(), strings.Repeat("a", 2*treeNonceBytes))
	for _, name := range []string{
		"foreign", valid + "-extra", strings.Replace(valid, "a", "g", 1),
		strings.Replace(valid, treePrefix, treePrefix+"0", 1), treePrefix + "0-" + strings.Repeat("a", 2*treeNonceBytes),
	} {
		if pid := treeCreator(name); pid != 0 {
			t.Errorf("creator %q = %d", name, pid)
		}
	}
	if pid := treeCreator(valid); pid != os.Getpid() {
		t.Fatalf("creator = %d", pid)
	}
	if treeProcessGone(os.Getpid()) || treeProcessGone(0) || treeProcessGone(-1) {
		t.Fatal("live or invalid process was considered gone")
	}
}

func TestTreeParentAcceptsUnrelatedHeadFile(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	setupTreeRepository(t, parent, "bare")
	if err := os.WriteFile(filepath.Join(parent, "HEAD"), []byte("project overview\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tree, err := newTree(filepath.Join(parent, "trees"))
	if err != nil {
		t.Fatalf("unrelated directory refused: %v", err)
	}
	if err := tree.Close(); err != nil {
		t.Fatal(err)
	}
}
