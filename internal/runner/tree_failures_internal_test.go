package runner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTreeCleanupFailureRetainsOwnership(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("directory permission removal is not supported on Windows")
	}
	tree, err := newTree(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(tree.Dir, "retained")
	if writeErr := os.WriteFile(file, []byte("keep"), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if chmodErr := os.Chmod(tree.Dir, 0); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	t.Cleanup(func() {
		if restoreErr := os.Chmod(tree.Dir, 0o700); restoreErr != nil && !errors.Is(restoreErr, os.ErrNotExist) {
			t.Error(restoreErr)
		}
	})
	if _, readErr := os.ReadFile(file); readErr == nil {
		t.Skip("directory permissions do not restrict this process")
	}
	if closeErr := tree.Close(); !errors.Is(closeErr, os.ErrPermission) {
		t.Fatalf("cleanup failure = %v", closeErr)
	}
	envelope := filepath.Dir(tree.Dir)
	data, err := os.ReadFile(filepath.Join(envelope, treeOwner))
	if err != nil || string(data) != treeOwnership(filepath.Base(envelope)) {
		t.Fatalf("ownership after failed cleanup = %q, %v", data, err)
	}
	if err := os.Chmod(tree.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := tree.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTreeOperationsPropagateUnavailableParent(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"regular-file", "cyclic-link"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			parent := unavailableTreeParent(t, kind)
			if tree, err := newTree(parent); tree != nil || err == nil {
				t.Fatalf("unavailable parent accepted = %v, %v", tree, err)
			}
			if err := Sweep(parent); err == nil {
				t.Fatal("sweep hid an unavailable parent")
			}
			if _, err := os.Lstat(parent); err != nil {
				t.Fatalf("parent changed: %v", err)
			}
		})
	}
}

// unavailableTreeParent is a parent no tree can be created under: a regular
// file, or a link to itself.
func unavailableTreeParent(t *testing.T, kind string) string {
	t.Helper()
	parent := filepath.Join(t.TempDir(), "parent")
	if kind == "regular-file" {
		if err := os.WriteFile(parent, []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		return parent
	}
	if err := os.Symlink("parent", parent); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	return parent
}

func TestTreeCloseRefusesInvalidOwnershipMetadata(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"oversized", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			tree, err := newTree(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			owner := filepath.Join(filepath.Dir(tree.Dir), treeOwner)
			if err := os.Remove(owner); err != nil {
				t.Fatal(err)
			}
			invalidTreeOwner(t, owner, kind)
			if err := tree.Close(); !errors.Is(err, os.ErrInvalid) {
				t.Fatalf("invalid owner result = %v", err)
			}
			if _, err := os.Stat(tree.Dir); err != nil {
				t.Fatalf("unverified payload changed: %v", err)
			}
		})
	}
}

func invalidTreeOwner(t *testing.T, path, kind string) {
	t.Helper()
	switch kind {
	case "oversized":
		if err := os.WriteFile(path, []byte(strings.Repeat("x", maxTreeOwnerBytes+1)), 0o600); err != nil {
			t.Fatal(err)
		}
	case "directory":
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	case "symlink":
		if err := os.Symlink("missing", path); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
}

func TestTreeOperationsPreserveClosedHandleErrors(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	root, err := os.OpenRoot(parent)
	if err != nil {
		t.Fatal(err)
	}
	if closeErr := root.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if tree, createErr := createTree(root, parent); tree != nil || !errors.Is(createErr, os.ErrClosed) {
		t.Fatalf("create with closed parent = %v, %v", tree, createErr)
	}
	for _, operation := range []struct {
		name string
		run  func() error
	}{
		{"publish", func() error { return publishTree(root, "pending") }},
		{"sweep", func() error { return sweepTreeEnvelopes(root) }},
		{"cleanup", func() error { return clearTreeEnvelope(root) }},
	} {
		if runErr := operation.run(); !errors.Is(runErr, os.ErrClosed) {
			t.Errorf("%s with closed parent = %v", operation.name, runErr)
		}
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("closed parent changed = %v, %v", entries, err)
	}
}

func TestTreeCloseRejectsAnotherCreatorAndAcceptsRemovedParent(t *testing.T) {
	t.Parallel()
	parent := filepath.Join(t.TempDir(), "trees")
	tree, err := newTree(parent)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("%s%d-%s", treePrefix, os.Getpid()+1, strings.Repeat("a", 2*treeNonceBytes))
	foreign := &Tree{Dir: filepath.Join(parent, name, treePayload)}
	if err := foreign.Close(); !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("other creator accepted: %v", err)
	}
	if err := tree.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	if err := tree.Close(); err != nil {
		t.Fatalf("already removed parent: %v", err)
	}
}

func TestTreeParentAcceptsHeadWithoutRepositoryObjects(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, treeHead), []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tree, err := newTree(parent)
	if err != nil {
		t.Fatalf("nonrepository parent refused: %v", err)
	}
	if err := tree.Close(); err != nil {
		t.Fatal(err)
	}
}
