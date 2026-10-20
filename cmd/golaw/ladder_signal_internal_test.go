package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
)

// signalBinding keeps the state of a ladder helper under home, so the test
// that interrupts the helper reads the trees the helper wrote.
func signalBinding(b binding, home string) binding {
	values := map[string]string{
		"HOME": home, "XDG_STATE_HOME": filepath.Join(home, "state"),
		"XDG_CONFIG_HOME": filepath.Join(home, "config"),
	}
	b.env = func(name string) string { return values[name] }
	return b
}

// TestLadderSignalChild runs one ladder helper in a process of its own for
// the test that interrupts it, and records the exit code and the diagnostic
// under home, because only a test main may exit with that code.
func TestLadderSignalChild(t *testing.T) {
	t.Parallel()
	if len(os.Args) < 4 || os.Args[len(os.Args)-3] != "golaw-signal" {
		return
	}
	command, home := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	ta := ladderApp(t)
	b := signalBinding(ta.bind(), home)
	ta.bind = func() binding { return b }
	args := []string{"affected"}
	if command == "lock" {
		args = []string{"lock", "measure", "--", "sleep", "30"}
	}
	result := fmt.Sprintf("%d\n%s", ta.run(t.Context(), args), ta.stderr.String())
	if err := os.WriteFile(filepath.Join(home, "result"), []byte(result), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The helper runs in a child process, because the signal has to reach the
// process that bound it; Windows sends a child neither signal.
func TestLadderHelpersStopCleanlyOnASignal(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows sends a child process no interrupt or termination signal")
	}
	for _, command := range []string{"affected", "lock"} {
		for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
			t.Run(command+"-"+sig.String(), func(t *testing.T) {
				t.Parallel()
				repo := signalRepo(t, command)
				home := t.TempDir()
				ready := signalReady(t, command, repo, home)
				child, log := startSignalChild(t, command, repo.Root, home)
				waitSignalReady(t, ready, home, log)
				if err := child.Process.Signal(sig); err != nil {
					t.Fatal(err)
				}
				if err := child.Wait(); err != nil {
					t.Fatalf("the helper died by the %s signal: %v\n%s", sig, err, readSignalFile(t, log))
				}
				requireSignalResult(t, home, log)
				requireSignalCleanup(t, command, repo, home)
			})
		}
	}
}

// signalRepo is a repository with a staged change, the work of golaw
// affected, or an empty one for golaw lock.
func signalRepo(t *testing.T, command string) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	if command == "affected" {
		repo.Commit("chore: seed", map[string]string{
			"go.mod":   "module example.test/signal\n\ngo 1.24\n",
			"value.go": "package signal\nconst Value = 1\n",
		})
		repo.Stage(map[string]string{"value.go": "package signal\nconst Value = 2\n"})
	}
	return repo
}

// signalReady answers whether the helper holds what it must clean up: a
// staged tree for golaw affected, the lock for golaw lock.
func signalReady(t *testing.T, command string, repo *gittest.Repo, home string) func() bool {
	t.Helper()
	if command == "lock" {
		pid := runner.LockPath(repo.GitDir(), "measure") + ".pid"
		return func() bool {
			_, err := os.Stat(pid)
			return err == nil
		}
	}
	trees := signalTrees(t, home)
	return func() bool {
		entries, err := os.ReadDir(trees)
		return err == nil && len(entries) > 0
	}
}

func signalTrees(t *testing.T, home string) string {
	t.Helper()
	dirs, err := signalBinding(ladderApp(t).bind(), home).dirs()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dirs.State, "runner", "trees")
}

// startSignalChild starts the test binary as the helper with a go on PATH
// that never answers, so golaw affected waits with its tree in place.
func startSignalChild(t *testing.T, command, root, home string) (*exec.Cmd, string) {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err = os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "child.log")
	output, err := os.Create(log)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)
	child := exec.CommandContext(ctx, program,
		"-test.run=^TestLadderSignalChild$", "--", "golaw-signal", command, home)
	child.Dir, child.Stdout, child.Stderr = root, output, output
	child.Env = append(gittest.Isolated(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err = child.Start(); err != nil {
		t.Fatal(errors.Join(err, output.Close()))
	}
	t.Cleanup(func() {
		stopSignalChild(t, child)
		if err := output.Close(); err != nil {
			t.Error(err)
		}
	})
	return child, log
}

// stopSignalChild reaps a child the test left running when it failed.
func stopSignalChild(t *testing.T, child *exec.Cmd) {
	t.Helper()
	if child.ProcessState != nil {
		return
	}
	if err := child.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Error(err)
	}
	var exited *exec.ExitError
	if err := child.Wait(); err != nil && !errors.As(err, &exited) {
		t.Error(err)
	}
}

// waitSignalReady polls, because no goroutine may wait on the child, until
// the helper is ready; a result before that is a helper that never waited.
func waitSignalReady(t *testing.T, ready func() bool, home, log string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	for !ready() {
		if _, err := os.Stat(filepath.Join(home, "result")); err == nil {
			t.Fatalf("the helper ended before the signal:\n%s", readSignalFile(t, log))
		}
		select {
		case <-ctx.Done():
			t.Fatalf("the helper never got ready: %v\n%s", ctx.Err(), readSignalFile(t, log))
		case <-time.After(time.Millisecond):
		}
	}
}

// requireSignalResult requires the helper to have answered exit 2 naming
// the cancellation the signal caused.
func requireSignalResult(t *testing.T, home, log string) {
	t.Helper()
	result := readSignalFile(t, filepath.Join(home, "result"))
	code, diagnostic, _ := strings.Cut(result, "\n")
	if code != strconv.Itoa(exitFailure) || !strings.Contains(diagnostic, context.Canceled.Error()) {
		t.Fatalf("result = %q\n%s", result, readSignalFile(t, log))
	}
}

// requireSignalCleanup requires the staged tree removed, or the lock and
// its pid file released.
func requireSignalCleanup(t *testing.T, command string, repo *gittest.Repo, home string) {
	t.Helper()
	if command == "lock" {
		pid := runner.LockPath(repo.GitDir(), "measure") + ".pid"
		if _, err := os.Stat(pid); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("pid file left behind: %v", err)
		}
		requireLockReleased(t, repo.GitDir())
		return
	}
	entries, err := os.ReadDir(signalTrees(t, home))
	if err != nil || len(entries) != 0 {
		t.Fatalf("resources remain = %v, %v", entries, err)
	}
}

func readSignalFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
