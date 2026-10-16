package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// crlfCheckout checks legacy.md out again under an end of line rule, so its
// bytes on disk end their lines with a carriage return while git stores
// them without.
func crlfCheckout(t *testing.T, root string, attribute bool) {
	t.Helper()
	if attribute {
		writeFiles(t, root, map[string]string{".gitattributes": "*.md text eol=crlf\n"})
		gitRun(t, root, "add", ".gitattributes")
		gitRun(t, root, "commit", "-q", "-m", "test: the end of line rule")
	} else {
		gitRun(t, root, "config", "core.autocrlf", "true")
	}
	if err := os.Remove(filepath.Join(root, "legacy.md")); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "checkout", "--", "legacy.md")
}

// filtered commits notes.txt under a clean filter that stores its letters
// in upper case, so the bytes on disk are never the blob git stores.
func filtered(t *testing.T, root, content string) {
	t.Helper()
	writeFiles(t, root, map[string]string{".gitattributes": "*.txt filter=upper\n", "notes.txt": content})
	gitRun(t, root, "config", "filter.upper.clean", "tr a-z A-Z")
	gitRun(t, root, "config", "filter.upper.smudge", "cat")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "test: the filtered notes")
}

// convertCases are commands around a file git converts on its way in, by
// an end of line rule or a filter, holding a dash the tree already had:
// the commands that change only its status leave it unjudged, while an edit
// of it is judged.
func convertCases(t *testing.T, dash string) []statusCase {
	t.Helper()
	crlf := func(root string) { crlfCheckout(t, root, false) }
	return []statusCase{
		{"a mode change under core.autocrlf", crlf, func(root string) { chmodFile(t, root, "legacy.md", 0o755) }, false},
		{"a mode change under an eol attribute", func(root string) { crlfCheckout(t, root, true) }, func(root string) { chmodFile(t, root, "legacy.md", 0o755) }, false},
		{"an index removal under core.autocrlf", crlf, func(root string) { gitRun(t, root, "rm", "-q", "--cached", "legacy.md") }, false},
		{"a soft reset under core.autocrlf", crlf, func(root string) { gitRun(t, root, "reset", "-q", "--soft", "HEAD~1") }, false},
		{"a mode change of a filtered file", func(root string) { filtered(t, root, dash) }, func(root string) { chmodFile(t, root, "notes.txt", 0o755) }, false},
		{"an edit under core.autocrlf", crlf, func(root string) {
			writeFiles(t, root, map[string]string{"legacy.md": "one \u2014 two\r\nthree \u2014 four\r\n"})
		}, true},
		{"an edit of a filtered file", func(root string) { filtered(t, root, "clean\n") }, func(root string) {
			writeFiles(t, root, map[string]string{"notes.txt": dash})
		}, true},
	}
}

// TestHookPostLeavesWhatAConversionOnlyChangedTheStatusOf holds the near
// misses of change detection over a file git converts on its way in,
// through the real binding: its bytes on disk are never the blob git
// stores, so git names the bytes as it would store them before a command
// that only moved its status is said to have changed it, and a dash already
// in it is not fed back, while an edit of it is.
func TestHookPostLeavesWhatAConversionOnlyChangedTheStatusOf(t *testing.T) {
	t.Parallel()
	dash := "one \u2014 two\n"
	agent := agentUnderTest()
	_, err := exec.LookPath("tr")
	for _, tc := range convertCases(t, dash) {
		if err != nil && strings.Contains(tc.name, "filtered") {
			continue
		}
		e := enrolledHook(t)
		commitFiles(t, e.root, map[string]string{"legacy.md": dash})
		tc.before(e.root)
		pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
		tc.during(e.root)
		post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
		said := string(post.Stdout)
		fed := strings.Contains(said, `"decision":"block"`) && strings.Contains(said, "text.dashes")
		if pre.ExitCode != 0 || len(pre.Stdout) != 0 || post.ExitCode != 0 || fed != tc.fed {
			t.Errorf("%s: pre %+v, post exit %d, stdout %q, stderr %q", tc.name, pre, post.ExitCode, said, post.Stderr)
		}
	}
}
