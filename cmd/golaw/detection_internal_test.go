package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
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
// docs/agent-contracts.md records, through the change listing of the real
// binding: git status names a link or a submodule that changed against the
// index, and the snapshot identifies a link by its class and the length of
// its text and a submodule by its class, so a link clean before the
// command and retargeted is reported, while a link already changed and
// retargeted again to a text of the same length and a submodule moved to
// another revision are not; a link retargeted to a text of another length
// is reported.
func TestDetectionExcludesWhatTheSnapshotDoesNotHold(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
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
	for _, step := range []struct {
		name, text string
		move       bool
		want       []string
	}{
		{"a clean link retargeted", "bb.md", false, []string{"link"}},
		{"a changed link retargeted to the same length and a moved submodule", "cc.md", true, nil},
		{"a link retargeted to another length", "longer.md", false, []string{"link"}},
	} {
		before := statusIdentities(t, h, root)
		retarget(t, link, step.text)
		if step.move {
			gitRun(t, module, append(identity, "commit", "-q", "--allow-empty", "-m", "build: two")...)
		}
		if changed := hook.Changed(before, statusIdentities(t, h, root)); !slices.Equal(changed, step.want) {
			t.Fatalf("%s: detection reported %v, want %v", step.name, changed, step.want)
		}
	}
}
