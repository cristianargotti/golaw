package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
)

// TestHookNamesAFileOverTheBoundACommandChanged drives a shell command
// around a tracked file over the read bound through the real binding: a
// rewrite of the same size, a growth and a changed modification time are
// named in the notice, while a dirty or clean file the command leaves
// alone answers the plain allow.
func TestHookNamesAFileOverTheBoundACommandChanged(t *testing.T) { //nolint:paralleltest // a file of 8 MiB is read alone, so the deadline measures the hook, not the tests beside it
	agent := agentUnderTest()
	line := "insert into t values (1);\n"
	big := strings.Repeat(line, hook.MaxInputBytes/len(line)+1)
	rewrite := func(root, content string) {
		rewriteInPlace(t, filepath.Join(root, "big.sql"), strings.Replace(content, "(1)", "(3)", 1))
	}
	for _, tc := range []struct {
		name         string
		dirty, named bool
		change       func(root, content string)
	}{
		{"a same size rewrite of a clean file", false, true, rewrite},
		{"a same size rewrite of a dirty file", true, true, rewrite},
		{"a dirty file that grows", true, true, func(root, content string) {
			writeFiles(t, root, map[string]string{"big.sql": content + line})
		}},
		{"a dirty file with another modification time", true, true, func(root, _ string) {
			stampFile(t, root, "big.sql", time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC))
		}},
		{"a dirty file the command leaves", true, false, func(string, string) {}},
		{"a clean file the command leaves", false, false, func(string, string) {}},
	} {
		e := enrolledHook(t)
		commitFiles(t, e.root, map[string]string{"big.sql": big})
		dirty := big
		if tc.dirty {
			dirty = big + "insert into t values (2);\n"
			writeFiles(t, e.root, map[string]string{"big.sql": dirty})
		}
		pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
		if pre.ExitCode != 0 || len(pre.Stdout) != 0 || pre.Stderr != "" {
			t.Fatalf("%s baseline: %+v", tc.name, pre)
		}
		tc.change(e.root, dirty)
		post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
		named := strings.Contains(string(post.Stdout), "golaw did not measure big.sql")
		if post.ExitCode != 0 || post.Stderr != "" || named != tc.named || (!tc.named && len(post.Stdout) != 0) {
			t.Errorf("%s: post exit %d, stdout %q, stderr %q", tc.name, post.ExitCode, post.Stdout, post.Stderr)
		}
	}
}
