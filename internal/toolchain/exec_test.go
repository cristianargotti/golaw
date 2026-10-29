package toolchain_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

func TestTheBudgetsFitInsideTheHookBudget(t *testing.T) {
	t.Parallel()
	const internalBudget = 8 * time.Second // internal/hook/run.go
	if toolchain.HookToolBudget != 1500*time.Millisecond || toolchain.HookEventBudget != 3*time.Second || toolchain.HookEventBudget >= internalBudget {
		t.Errorf("budgets = %v per tool, %v per event", toolchain.HookToolBudget, toolchain.HookEventBudget)
	}
}

func TestExecPipesTheBytesOnStdinAndAnswersTheOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := runnertest.Install(t, dir, "ruff", runnertest.Behavior{Stdout: "[]\n", Stdin: true})
	args := []string{"check", "--stdin-filename", "scripts/a.py", "-"}
	got, err := toolchain.Exec(t.Context(), toolchain.Request{
		Path: path, Args: args, Stdin: []byte("import os\n"), Dir: dir, Timeout: toolchain.HookToolBudget,
	})
	if err != nil || got.Killed || got.Output.Code != 0 || string(got.Output.Stdout) != "[]\n" || got.Elapsed != 0 {
		t.Fatalf("Exec = %+v, %v", got, err)
	}
	if inputs := runnertest.Inputs(t, dir, "ruff"); len(inputs) != 1 || string(inputs[0]) != "import os\n" {
		t.Errorf("stdin = %q, want the written bytes once", inputs)
	}
	if calls := runnertest.Calls(t, dir, "ruff"); len(calls) != 1 || !slices.Equal(calls[0], args) {
		t.Errorf("calls = %v, want %v", calls, args)
	}
}

func TestExecKillsAToolThatPassesItsBudget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := runnertest.Install(t, dir, "shellcheck", runnertest.Behavior{Stdout: "{}\n", Delay: "20s"})
	got, err := toolchain.Exec(t.Context(), toolchain.Request{
		Path: path, Args: []string{"--format", "json1", "-"}, Dir: dir, Timeout: 200 * time.Millisecond, Clock: time.Now,
	})
	if err != nil || !got.Killed {
		t.Fatalf("Exec = %+v, %v, want a kill and no error", got, err)
	}
	if got.Elapsed <= 0 || got.Elapsed > 10*time.Second {
		t.Errorf("elapsed = %v, want the measured wall time of the kill", got.Elapsed)
	}
}

func TestExecReportsWhatItCouldNotRun(t *testing.T) {
	t.Parallel()
	if _, err := toolchain.Exec(t.Context(), toolchain.Request{Path: filepath.Join(t.TempDir(), "absent")}); !errors.Is(err, runner.ErrStart) {
		t.Errorf("absent tool = %v, want ErrStart", err)
	}
	dir := t.TempDir()
	path := runnertest.Install(t, dir, "hadolint", runnertest.Behavior{Stdout: "[]\n", Delay: "20s"})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := toolchain.Exec(ctx, toolchain.Request{Path: path, Dir: dir, Timeout: toolchain.HookToolBudget})
	if !errors.Is(err, context.Canceled) || got.Killed {
		t.Errorf("canceled event = %+v, %v, want the cancellation, not a kill", got, err)
	}
}

func TestToolEnvKeepsOnlyThePathHomeTemporaryAndLocaleVariables(t *testing.T) {
	t.Parallel()
	environ := []string{
		"PATH=/usr/bin", "HOME=/h", "TMPDIR=/t", "LANG=pt_BR.UTF-8", "LANGUAGE=pt_BR", "LC_ALL=C", "LC_CTYPE=UTF-8",
		"HTTPS_PROXY=http://proxy.test", "NODE_OPTIONS=--require=/x.js", "PYTHONPATH=/p", "RUFF_CACHE_DIR=/c", "GIT_DIR=/r/.git",
		`Path=C:\bin`, `SystemRoot=C:\Windows`, `TEMP=C:\T`, `TMP=C:\T`, `USERPROFILE=C:\U`, "NOEQUALS",
	}
	want := []string{
		"PATH=/usr/bin", "HOME=/h", "TMPDIR=/t", "LANG=pt_BR.UTF-8", "LANGUAGE=pt_BR", "LC_ALL=C", "LC_CTYPE=UTF-8",
		`Path=C:\bin`, `SystemRoot=C:\Windows`, `TEMP=C:\T`, `TMP=C:\T`, `USERPROFILE=C:\U`,
	}
	if got := toolchain.ToolEnv(environ); !slices.Equal(got, want) {
		t.Errorf("ToolEnv = %v, want %v", got, want)
	}
	if got := toolchain.ToolEnv(nil); got == nil || len(got) != 0 {
		t.Errorf("ToolEnv(nil) = %#v, want an empty environment, never the inherited one", got)
	}
}
