package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// retarget points a symbolic link at another text, never at the file.
func retarget(t *testing.T, link, text string) {
	t.Helper()
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(text, link); err != nil {
		t.Fatal(err)
	}
}

// TestDetectionExcludesWhatTheSnapshotDoesNotHold pins the exclusion
// docs/agent-contracts.md records, through the real worktree snapshot: it
// holds neither the text of a symbolic link nor the revision a submodule
// has checked out, so a link retargeted to a text of the same length and a
// submodule moved to another revision keep their identity and change
// detection reports neither, while a link retargeted to a text of another
// length is reported.
func TestDetectionExcludesWhatTheSnapshotDoesNotHold(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	module := filepath.Join(root, "mod")
	if err := os.MkdirAll(module, 0o700); err != nil {
		t.Fatal(err)
	}
	identity := []string{"-c", "user.email=tester@example.test", "-c", "user.name=tester"}
	gitRun(t, module, "init", "-q")
	gitRun(t, module, append(identity, "commit", "-q", "--allow-empty", "-m", "build: one")...)
	link := filepath.Join(root, "link")
	if err := os.Symlink("aa.md", link); err != nil {
		t.Skipf("symbolic links are not available: %v", err)
	}
	gitRun(t, root, "add", "link", "mod")
	gitRun(t, root, "commit", "-q", "-m", "build: a link and a submodule")
	identities := func() map[string]string {
		snapshot, err := b.snapshot(t.Context(), root)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Class("link") != model.ClassSymlink || snapshot.Class("mod") != model.ClassGitlink {
			t.Fatalf("the snapshot lists the link as %s and the submodule as %s", snapshot.Class("link"), snapshot.Class("mod"))
		}
		got, err := hook.Identities(t.Context(), snapshot, nil)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	before := identities()
	retarget(t, link, "bb.md")
	gitRun(t, module, append(identity, "commit", "-q", "--allow-empty", "-m", "build: two")...)
	if changed := hook.Changed(before, identities()); len(changed) != 0 {
		t.Fatalf("detection reported %v, which docs/agent-contracts.md records as excluded", changed)
	}
	retarget(t, link, "longer.md")
	if changed := hook.Changed(before, identities()); !slices.Equal(changed, []string{"link"}) {
		t.Fatalf("a link retargeted to a text of another length: %v", changed)
	}
}
