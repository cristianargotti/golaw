package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// One post event whose three external stages all meet a tool that never
// answers, in the order the hook runs them: the generator and the formatter
// are each killed at the 1.5 s limit of a tool, which spends the event's
// 3 s, so no linter starts and every file is given up. The event answers
// within its budget plus the time a stopped tool takes to exit, the
// generator named as the cause, the in-process fixes kept without the
// formatter; stages that each took a budget of their own would answer
// after 7.5 s.
func TestOnePostEventWithEverySlowStageAnswersWithinItsBudget(t *testing.T) {
	t.Parallel()
	h := regenBinding(t)
	h.budget = &eventBudget{}
	warmGenerators(t, h, runnertest.Behavior{Stdout: "ok", Delay: "5s"})
	installLinter(t, h, runnertest.Behavior{Stdout: linterDone, Stdin: true, Delay: "5s"})
	installFake(t, h.dirs.Tools, "shellcheck", runnertest.Behavior{Stdout: "{\"comments\":[]}", Delay: "5s"})
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(lintFmtPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(lintFmtInput), 0o600); err != nil {
		t.Fatal(err)
	}
	snap := proof.NewTree(map[string][]byte{
		"go.mod": []byte("module example.com/slice\n\ngo 1.27\n"), lintFmtPath: []byte(lintFmtInput), "sqlc.yaml": sqlcConfig,
	})
	shell := []hook.ToolFile{
		{Path: "scripts/a.sh", Content: []byte("#!/bin/sh\necho a\n"), Language: surface.Shell},
		{Path: "scripts/b.sh", Content: []byte("#!/bin/sh\necho b\n"), Language: surface.Shell},
		{Path: "scripts/c.sh", Content: []byte("#!/bin/sh\necho c\n"), Language: surface.Shell},
	}
	h.begin()
	start := time.Now()
	third := h.thirdParty(t.Context(), root, map[string]string{"sqlc.yaml": source.ContentHash(sqlcConfig)}, snap, []thirdparty.Generator{sqlcGenerator()})
	event, cancel := h.budget.context(t.Context())
	fixed := fixFiles(event, root, "example.com/slice", snap, []string{lintFmtPath}, h.lintFmt(root))
	cancel()
	report := h.tools(t.Context(), hook.Repository{Root: root}, proof.Policy(), shell)
	if elapsed := time.Since(start); elapsed > toolchain.HookEventBudget+time.Second {
		t.Fatalf("the event took %s, over its budget of %s", elapsed, toolchain.HookEventBudget)
	}
	if third.ran || !slices.Equal(third.causes, []string{"sqlc passed the hook's 1.5 s limit"}) {
		t.Errorf("generators = %+v, want sqlc killed at its limit and named", third)
	}
	if len(fixed) != 1 || !slices.Equal(fixed[0].parts, []string{"go/format", "imports fixed"}) {
		t.Errorf("fixed = %+v, want the in-process fixes alone once the formatter was killed", fixed)
	}
	if !slices.Equal(report.Killed, []string{"scripts/a.sh", "scripts/b.sh", "scripts/c.sh"}) || len(report.Findings) != 0 {
		t.Errorf("linters = %+v, want every file given up when the event ran out", report)
	}
}
