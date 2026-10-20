package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

func ladderProbePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(filepath.FromSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestLadderEnvironmentProbe(t *testing.T) {
	t.Parallel()
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "golaw-probe" {
		return
	}
	root := os.Args[len(os.Args)-1]
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if ladderProbePath(t, cwd) != ladderProbePath(t, root) {
		t.Fatalf("cwd = %q, want %q", cwd, root)
	}
	for _, name := range runner.HookVariables() {
		if _, exists := os.LookupEnv(name); exists {
			t.Fatalf("hook variable inherited: %s", name)
		}
	}
	if os.Getenv("LANG") != "C.UTF-8" {
		t.Fatal("ordinary environment variable was removed")
	}
	other := gittest.New(t)
	cmd := exec.CommandContext(t.Context(), "git", "rev-parse", "--show-toplevel")
	cmd.Dir = other.Root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if ladderProbePath(t, strings.TrimSpace(string(out))) !=
		ladderProbePath(t, other.Root) {
		t.Fatalf("child git answered for another repository: %q", out)
	}
}

//nolint:paralleltest // The command uses cwd and a deliberately contaminated environment.
func TestLockCommandStripsEveryHookVariable(t *testing.T) {
	repo := gittest.New(t)
	t.Chdir(repo.Root)
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ta := ladderApp(t)
	b := ta.bind()
	b.environ = func() []string {
		env := append(gittest.Isolated(), "LANG=C.UTF-8")
		for _, name := range runner.HookVariables() {
			env = append(env, name+"=must-not-reach-child")
		}
		return env
	}
	ta.bind = func() binding { return b }
	args := []string{
		"lock", "probe", "--", program,
		"-test.run=^TestLadderEnvironmentProbe$", "--", "golaw-probe", repo.Root,
	}
	if got := ta.run(t.Context(), args); got != exitClean {
		t.Fatalf("exit = %d, stdout=%q stderr=%q",
			got, ta.stdout.String(), ta.stderr.String())
	}
}

// A subdirectory as cwd tells the discovered worktree root from the
// directory the command was started in.
//
//nolint:paralleltest // The command resolves the repository from the process cwd.
func TestLockCommandRunsTheChildInTheWorktreeRoot(t *testing.T) {
	repo := gittest.New(t)
	repo.Write("sub/keep.txt", "keep\n")
	t.Chdir(filepath.Join(repo.Root, "sub"))
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ta := ladderApp(t)
	b := ta.bind()
	b.environ = func() []string { return append(gittest.Isolated(), "LANG=C.UTF-8") }
	ta.bind = func() binding { return b }
	args := []string{
		"lock", "probe", "--", program,
		"-test.run=^TestLadderEnvironmentProbe$", "--", "golaw-probe", repo.Root,
	}
	if got := ta.run(t.Context(), args); got != exitClean {
		t.Fatalf("exit = %d, stdout=%q stderr=%q",
			got, ta.stdout.String(), ta.stderr.String())
	}
}

func TestLockedInputChild(t *testing.T) {
	t.Parallel()
	if os.Getenv("GOLAW_LOCK_CHILD") != "1" {
		return
	}
	if _, err := os.Stdout.WriteString("ready\n"); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 1)
	if _, err := io.ReadFull(os.Stdin, data); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(os.Stdout, "%s\n", data); err != nil {
		t.Fatal(err)
	}
}

func lockChildApp(t *testing.T) (testApp, string) {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ta := ladderApp(t)
	b := ta.bind()
	b.environ = func() []string {
		return append(gittest.Isolated(), "GOLAW_LOCK_CHILD=1")
	}
	ta.bind = func() binding { return b }
	return ta, program
}

//nolint:paralleltest // The command resolves the repository from the process cwd.
func TestLockCommandPassesStdin(t *testing.T) {
	repo := gittest.New(t)
	t.Chdir(repo.Root)
	ta, program := lockChildApp(t)
	ta.stdin = strings.NewReader("x")
	args := []string{"lock", "measure", "--", program, "-test.run=^TestLockedInputChild$"}
	if code := ta.run(t.Context(), args); code != exitClean ||
		!strings.Contains(ta.stdout.String(), "ready\nx\n") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, ta.stdout.String(), ta.stderr.String())
	}
	requireLockReleased(t, repo.GitDir())
}

type cancelLockedOutput struct {
	cancel  context.CancelFunc
	started bool
}

func (w *cancelLockedOutput) Write(data []byte) (int, error) {
	w.started = true
	w.cancel()
	return len(data), nil
}

//nolint:paralleltest // The command resolves the repository from the process cwd.
func TestLockCommandCancelsAfterTheChildStarts(t *testing.T) {
	repo := gittest.New(t)
	t.Chdir(repo.Root)
	ta, program := lockChildApp(t)
	input, supply, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := errors.Join(input.Close(), supply.Close()); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	output := &cancelLockedOutput{cancel: cancel}
	ta.stdin, ta.app.stdout = input, output
	args := []string{"lock", "measure", "--", program, "-test.run=^TestLockedInputChild$"}
	if code := ta.run(ctx, args); code != exitFailure || !output.started ||
		!strings.Contains(ta.stderr.String(), context.Canceled.Error()) {
		t.Fatalf("exit = %d, started = %t, stderr = %q", code, output.started, ta.stderr.String())
	}
	requireLockReleased(t, repo.GitDir())
}
