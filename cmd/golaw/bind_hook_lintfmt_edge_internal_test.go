package main

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

// A linter that exits 0 but prints something other than Go source, such as
// a warning on stdout, must never replace the file the agent wrote.
func TestALinterThatPrintsNoGoSourceLeavesTheInProcessFixes(t *testing.T) {
	t.Parallel()
	for name, out := range map[string]string{
		"near: a warning on stdout": "level=warning msg=\"no formatter enabled\"\n",
		"near: half a file":         "package growth\n\nfunc label() string {\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := regenBinding(t)
			installLinter(t, h, runnertest.Behavior{Stdout: out})
			got, fixed := lintFmtRun(t, h)
			if got != inProcessDone || len(fixed) != 1 || slices.Contains(fixed[0].parts, partLintFmt) {
				t.Fatalf("file =\n%s\nfixed = %+v", got, fixed)
			}
		})
	}
}
