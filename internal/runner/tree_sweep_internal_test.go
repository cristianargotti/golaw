package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTreeOwnerExitChild(t *testing.T) { t.Parallel() }

func TestTreeSweepRecoversPartialPublication(t *testing.T) {
	t.Parallel()
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestTreeOwnerExitChild$")
	if err := child.Run(); err != nil {
		t.Fatal(err)
	}
	dead := child.ProcessState.Pid()
	if !treeProcessGone(dead) {
		t.Skip("ended process cannot be identified with certainty")
	}
	for _, tc := range []struct {
		name     string
		pid      int
		metadata string
		remove   bool
	}{
		{"red-before-publication", dead, "pending", true},
		{"near-live-publication", os.Getpid(), "pending", false},
		{"red-published", dead, "valid", true},
		{"near-malformed-owner", dead, "invalid", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			name := fmt.Sprintf("%s%d-%s", treePrefix, tc.pid, strings.Repeat("a", 2*treeNonceBytes))
			envelope := filepath.Join(parent, name)
			if err := os.Mkdir(envelope, 0o700); err != nil {
				t.Fatal(err)
			}
			writePartialTreeOwner(t, envelope, name, tc.metadata)
			if err := Sweep(parent); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(envelope)
			if tc.remove && !errors.Is(err, os.ErrNotExist) || !tc.remove && err != nil {
				t.Fatalf("sweep result = %v, remove=%t", err, tc.remove)
			}
		})
	}
}

func writePartialTreeOwner(t *testing.T, envelope, name, kind string) {
	t.Helper()
	file, content := treeOwner, "invalid"
	switch kind {
	case "valid":
		content = treeOwnership(name)
	case "pending":
		file = treeOwnerPending
	}
	if err := os.WriteFile(filepath.Join(envelope, file), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestTreeNewParentAndIdempotentClose(t *testing.T) {
	t.Parallel()
	parent := filepath.Join(t.TempDir(), "absent", "trees")
	tree, err := newTree(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{parent, filepath.Dir(tree.Dir), tree.Dir} {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatal(statErr)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("tree directory is not private: %s mode=%v", path, info.Mode())
		}
	}
	for range 2 {
		if closeErr := tree.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("resources remain = %v, %v", entries, err)
	}
}

func TestTreeCloseRefusesSubstitutedParent(t *testing.T) {
	t.Parallel()
	parent := filepath.Join(t.TempDir(), "trees")
	tree, err := newTree(parent)
	if err != nil {
		t.Fatal(err)
	}
	retained := parent + ".retained"
	if err := os.Rename(parent, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(retained, parent); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := tree.Close(); err == nil {
		t.Fatal("close followed a substituted parent")
	}
	name := filepath.Base(filepath.Dir(tree.Dir))
	if _, err := os.Stat(filepath.Join(retained, name, treePayload)); err != nil {
		t.Fatalf("substituted parent changed: %v", err)
	}
}
