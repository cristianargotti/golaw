package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// writeFiles writes files of a test under the root.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// commitFiles writes files under the root and commits them.
func commitFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	writeFiles(t, root, files)
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "test: the files of the case")
}

// rewriteInPlace replaces a file the way sed -i does: the new content is
// written beside it and renamed over it.
func rewriteInPlace(t *testing.T, path, content string) {
	t.Helper()
	temp := path + ".sed"
	if err := os.WriteFile(temp, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(temp, path); err != nil {
		t.Fatal(err)
	}
}

// statusIdentities is the content identity of what the binding the hook
// runs with reports changed in the repository.
func statusIdentities(t *testing.T, h hookBinding, root string) map[string]string {
	t.Helper()
	snapshot, paths, err := h.changes(t.Context(), hook.Repository{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	got, err := hook.Identities(t.Context(), snapshot, paths)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestHookDetectsWhatACommandChanged holds the red fixtures of change
// detection through the index: a file rewritten in place as sed -i does,
// one a redirection rewrote, a new untracked file, a deleted file, a file
// already dirty and written again, a file written back to its index
// content and a file the command staged are each reported, while a dirty
// file the command leaves alone and every clean file are not.
func TestHookDetectsWhatACommandChanged(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	h := hookBindingFor(t, b)
	root := repository(t, true)
	commitFiles(t, root, map[string]string{
		"sed.md": "one\n", "redirect.md": "one\n", "deleted.md": "one\n", "dirty.md": "one\n",
		"back.md": "one\n", "staged.md": "one\n", "kept.md": "one\n", "clean.md": "one\n",
	})
	writeFiles(t, root, map[string]string{"dirty.md": "two\n", "back.md": "two\n", "kept.md": "two\n"})
	before := statusIdentities(t, h, root)
	rewriteInPlace(t, filepath.Join(root, "sed.md"), "changed\n")
	writeFiles(t, root, map[string]string{
		"redirect.md": "changed\n", "fresh.md": "new\n", "dirty.md": "three\n", "back.md": "one\n", "staged.md": "changed\n",
	})
	if err := os.Remove(filepath.Join(root, "deleted.md")); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "staged.md")
	got := hook.Changed(before, statusIdentities(t, h, root))
	want := []string{"back.md", "deleted.md", "dirty.md", "fresh.md", "redirect.md", "sed.md", "staged.md"}
	if !slices.Equal(got, want) {
		t.Fatalf("detection reported %v, want %v", got, want)
	}
}

// TestHookPostJudgesWhatACommandWrote drives a Bash pre and post event
// through the real binding around what the command did: a dash a
// redirection wrote and a dash restored by writing a file back to its
// committed content are fed back naming the file, while a committed dash
// the command never touched is never read.
func TestHookPostJudgesWhatACommandWrote(t *testing.T) {
	t.Parallel()
	dash := "one \u2014 two\n"
	for _, tc := range []struct {
		name          string
		before, after map[string]string
		path          string
	}{
		{"a dash a redirection wrote", nil, map[string]string{"notes.md": dash}, "notes.md"},
		{"a dash written back to the committed content", map[string]string{"legacy.md": "clean\n"}, map[string]string{"legacy.md": dash}, "legacy.md"},
		{"a committed dash the command never touched", nil, map[string]string{"other.md": "clean\n"}, ""},
	} {
		e := enrolledHook(t)
		commitFiles(t, e.root, map[string]string{"legacy.md": dash})
		writeFiles(t, e.root, tc.before)
		agent := agentUnderTest()
		pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
		writeFiles(t, e.root, tc.after)
		post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
		said := string(post.Stdout)
		blocked := strings.Contains(said, `"decision":"block"`) && strings.Contains(said, "text.dashes")
		if pre.ExitCode != 0 || len(pre.Stdout) != 0 || post.ExitCode != 0 || blocked != (tc.path != "") || !strings.Contains(said, tc.path) {
			t.Fatalf("%s: pre %+v, post exit %d, stdout %q, stderr %q", tc.name, pre, post.ExitCode, said, post.Stderr)
		}
	}
}
