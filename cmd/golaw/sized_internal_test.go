package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
)

// TestHookNamesADirtyFileOverTheBoundAtBothEnds drives a shell command
// around a tracked file over the read bound through the real binding: while
// git status names the file before and after the command, a rewrite to the
// same size and no rewrite at all leave the same identity, its size, so the
// post event names the file in its notice either way, while a clean file
// the command leaves alone is not named.
func TestHookNamesADirtyFileOverTheBoundAtBothEnds(t *testing.T) { //nolint:paralleltest // a file of 8 MiB is read alone, so the deadline measures the hook, not the tests beside it
	agent := agentUnderTest()
	line := "insert into t values (1);\n"
	big := strings.Repeat(line, hook.MaxInputBytes/len(line)+1)
	for _, tc := range []struct {
		name                  string
		dirty, rewrite, named bool
	}{
		{"a same size rewrite of a dirty file", true, true, true},
		{"a dirty file the command leaves", true, false, true},
		{"a clean file the command leaves", false, false, false},
	} {
		e := enrolledHook(t)
		commitFiles(t, e.root, map[string]string{"big.sql": big})
		dirty := big
		if tc.dirty {
			dirty = big + "insert into t values (2);\n"
			writeFiles(t, e.root, map[string]string{"big.sql": dirty})
		}
		pre := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PreToolUse", e.root, "true")), agent.Adapter, e.deps)
		if tc.rewrite {
			rewriteInPlace(t, filepath.Join(e.root, "big.sql"), strings.Replace(dirty, "(1)", "(3)", 1))
		}
		post := hook.Run(t.Context(), agent.ID, strings.NewReader(shellEventAt(t, "PostToolUse", e.root, "true")), agent.Adapter, e.deps)
		named := strings.Contains(string(post.Stdout), "golaw did not measure big.sql")
		if pre.ExitCode != 0 || post.ExitCode != 0 || named != tc.named {
			t.Errorf("%s: pre %+v, post exit %d, stdout %q", tc.name, pre, post.ExitCode, post.Stdout)
		}
	}
}
