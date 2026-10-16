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
	changes, err := h.changes(t.Context(), hook.Repository{Root: root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := hook.Identities(t.Context(), changes.Snapshot, changes.Paths)
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

// statusCase is one command of a post detection fixture: what the
// repository holds before the pre event, what the command does and
// whether the post event feeds a dash back.
type statusCase struct {
	name           string
	before, during func(root string)
	fed            bool
}

// statusCases are the near misses and the controls of a command that
// moves a file in or out of what git status reports.
func statusCases(t *testing.T, dash string) []statusCase {
	t.Helper()
	commit := func(root, message string) { gitRun(t, root, "commit", "-q", "-am", message) }
	return []statusCase{
		{"a committed dash file made executable", nil, func(root string) { chmodFile(t, root, "legacy.md", 0o755) }, false},
		{"a committed executable dash file made plain", func(root string) {
			chmodFile(t, root, "legacy.md", 0o755)
			commit(root, "test: executable")
		}, func(root string) { chmodFile(t, root, "legacy.md", 0o644) }, false},
		{"dirty dash bytes the command commits unchanged", func(root string) {
			writeFiles(t, root, map[string]string{"notes.md": dash})
			gitRun(t, root, "add", "notes.md")
		}, func(root string) { commit(root, "test: commit the notes") }, false},
		{"an untracked dash file the command adds and commits", func(root string) {
			writeFiles(t, root, map[string]string{"draft.md": dash})
		}, func(root string) {
			gitRun(t, root, "add", "-A")
			commit(root, "test: commit the draft")
		}, false},
		{"a committed dash file a soft reset leaves staged", func(root string) {
			commitFiles(t, root, map[string]string{"other.md": dash})
		}, func(root string) { gitRun(t, root, "reset", "-q", "--soft", "HEAD~1") }, false},
		{"a dash written into a clean file", nil, func(root string) { writeFiles(t, root, map[string]string{"clean.md": dash}) }, true},
		{"a dirty file written back to its committed dash", func(root string) {
			writeFiles(t, root, map[string]string{"legacy.md": "clean\n"})
		}, func(root string) { writeFiles(t, root, map[string]string{"legacy.md": dash}) }, true},
	}
}

// TestHookPostLeavesWhatOnlyChangedItsStatus holds the near misses of
// change detection through the real binding: a command that only moves a
// file in or out of what git status reports, a mode change, a commit of
// bytes already dirty, a soft reset that leaves a committed file staged,
// leaves its content alone, so a dash already in it is not fed back, while
// a dash written into a clean file and a file written back to its
// committed content are.
func TestHookPostLeavesWhatOnlyChangedItsStatus(t *testing.T) {
	t.Parallel()
	dash := "one \u2014 two\n"
	agent := agentUnderTest()
	for _, tc := range statusCases(t, dash) {
		e := enrolledHook(t)
		commitFiles(t, e.root, map[string]string{"legacy.md": dash, "clean.md": "clean\n"})
		if tc.before != nil {
			tc.before(e.root)
		}
		pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
		tc.during(e.root)
		post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
		said := string(post.Stdout)
		fed := strings.Contains(said, `"decision":"block"`) && strings.Contains(said, "text.dashes")
		if pre.ExitCode != 0 || len(pre.Stdout) != 0 || post.ExitCode != 0 || fed != tc.fed {
			t.Fatalf("%s: pre %+v, post exit %d, stdout %q, stderr %q", tc.name, pre, post.ExitCode, said, post.Stderr)
		}
	}
}

// chmodFile sets the mode of a file of a test under the root.
func chmodFile(t *testing.T, root, name string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(filepath.Join(root, filepath.FromSlash(name)), mode); err != nil {
		t.Fatal(err)
	}
}
