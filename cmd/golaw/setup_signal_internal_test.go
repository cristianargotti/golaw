package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/setup"
)

const foreignHook = "#!/bin/sh\necho foreign hook\n"

// TestSetupSignalChild runs golaw setup in a process of its own for the
// test that terminates it, with its state under home, and records the exit
// code and the diagnostic there, because only a test main may exit.
func TestSetupSignalChild(t *testing.T) {
	t.Parallel()
	if len(os.Args) < 4 || os.Args[len(os.Args)-3] != "golaw-setup-signal" {
		return
	}
	root, home := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	ta := bindApp(t)
	b := signalBinding(ta.bind(), home)
	ta.bind = func() binding { return b }
	result := fmt.Sprintf("%d\n%s", ta.run(t.Context(), []string{"setup", root}), ta.stderr.String())
	if err := os.WriteFile(filepath.Join(home, "result"), []byte(result), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A termination that reaches setup after lefthook wrote the hooks puts the
// hooks back, the foreign one included, and leaves no receipt, so the next
// setup arms the rungs. The setup runs in a child process, because the
// signal has to reach the process that bound it; Windows sends a child no
// such signal.
func TestSetupPutsTheHooksBackOnASignal(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Windows sends a child process no interrupt or termination signal")
	}
	for _, sig := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()
			repo, bin, home := setupSignalRepo(t), t.TempDir(), t.TempDir()
			hooks := filepath.Join(repo.GitDir(), "hooks")
			if err := os.MkdirAll(hooks, 0o750); err != nil {
				t.Fatal(err)
			}
			writeSignalFile(t, filepath.Join(hooks, "pre-commit"), foreignHook, 0o700)
			setupSignalTools(t, bin, hooks)
			signalSetup(t, sig, bin, repo.Root, home)
			requireHooks(t, hooks, map[string]string{"pre-commit": foreignHook})
			if receipt := setupSignalReceipt(t, repo.Root, home); receipt != "" {
				t.Fatalf("a receipt after the signal: %s", receipt)
			}
			retrySetup(t, bin, repo.Root, home)
			requireHooks(t, hooks, map[string]string{
				"pre-commit": lefthookFakeHook, "commit-msg": lefthookFakeHook,
				"pre-push": lefthookFakeHook, "pre-commit.old": foreignHook,
			})
		})
	}
}

// signalSetup starts setup with a lefthook that blocks after it wrote the
// hooks, sends the signal, and requires the exit code and the reason that
// names the cancellation and the restored hooks.
func signalSetup(t *testing.T, sig os.Signal, bin, root, home string) {
	t.Helper()
	writeSignalFile(t, filepath.Join(bin, "block"), "", 0o600)
	ready := func() bool {
		_, err := os.Stat(filepath.Join(bin, "ready"))
		return err == nil
	}
	child, log := startSetupChild(t, bin, root, home)
	waitSignalReady(t, ready, home, log)
	if err := child.Process.Signal(sig); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err != nil {
		t.Fatalf("setup died by the %s signal: %v\n%s", sig, err, readSignalFile(t, log))
	}
	requireSetupResult(t, home, log, exitFailure, context.Canceled.Error(), "setup put the git hooks back")
}

// retrySetup runs setup again with a lefthook that finishes, and requires
// it to arm the rungs and write the receipt.
func retrySetup(t *testing.T, bin, root, home string) {
	t.Helper()
	for _, name := range []string{filepath.Join(bin, "block"), filepath.Join(home, "result")} {
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
	}
	retry, log := startSetupChild(t, bin, root, home)
	if err := retry.Wait(); err != nil {
		t.Fatalf("retry: %v\n%s", err, readSignalFile(t, log))
	}
	requireSetupResult(t, home, log, exitClean)
	if setupSignalReceipt(t, root, home) == "" {
		t.Fatal("the retry wrote no receipt")
	}
}

// setupSignalRepo is a repository adopted with the kit policy and a
// lefthook.yml that routes every rung.
func setupSignalRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.New(t)
	repo.Commit("chore: establish policy", map[string]string{
		"golaw.yml": string(renderedPolicy(t)),
		"go.mod":    "module example.test/service\n\ngo 1.27\n",
		"lefthook.yml": "pre-commit:\n  commands:\n    golaw:\n      run: golaw hook pre-commit\n" +
			"commit-msg:\n  commands:\n    golaw:\n      run: golaw hook commit-msg '{1}'\n" +
			"pre-push:\n  commands:\n    golaw:\n      run: golaw hook pre-push '{1}' '{2}'\n",
	})
	return repo
}

// lefthookFakeHook is the hook the fake lefthook writes.
const lefthookFakeHook = "#!/bin/sh\n# lefthook fake\n"

// setupSignalTools installs every pinned tool as a fake that answers its
// probe, and a lefthook that writes the three hooks as lefthook does, the
// foreign one moved to .old, then marks ready and waits while block exists.
func setupSignalTools(t *testing.T, bin, hooks string) {
	t.Helper()
	for _, tool := range runner.Toolchain() {
		runnertest.Install(t, bin, tool.Name, runnertest.Behavior{Probe: tool.Probe, Version: tool.Version})
	}
	runnertest.Install(t, bin, "golaw", runnertest.Behavior{Probe: []string{"version"}})
	tool, _ := runner.LookupTool("lefthook")
	script := "#!/bin/sh\nPATH=/usr/bin:/bin\nexport PATH\nhooks='" + hooks + "'\nbin='" + bin + "'\n" +
		"if [ \"$1\" = " + tool.Probe[0] + " ]; then echo " + tool.Version + "; exit 0; fi\n" +
		"for hook in pre-commit commit-msg pre-push; do\n" +
		"  if [ -f \"$hooks/$hook\" ] && ! grep -q 'lefthook fake' \"$hooks/$hook\"; then mv \"$hooks/$hook\" \"$hooks/$hook.old\"; fi\n" +
		"  printf '#!/bin/sh\\n# lefthook fake\\n' > \"$hooks/$hook\"\n" +
		"done\n" +
		"if [ -f \"$bin/block\" ]; then : > \"$bin/ready\"; exec sleep 30; fi\n"
	path := filepath.Join(bin, "lefthook")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	writeSignalFile(t, path, script, 0o700)
}

// startSetupChild starts the test binary as golaw setup with the fakes
// first on PATH.
func startSetupChild(t *testing.T, bin, root, home string) (*exec.Cmd, string) {
	t.Helper()
	program, err := os.Executable()
	if err != nil {
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
		"-test.run=^TestSetupSignalChild$", "--", "golaw-setup-signal", root, home)
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

// requireSetupResult requires the exit code setup answered and every text
// of its diagnostic.
func requireSetupResult(t *testing.T, home, log string, want int, texts ...string) {
	t.Helper()
	result := readSignalFile(t, filepath.Join(home, "result"))
	code, diagnostic, _ := strings.Cut(result, "\n")
	if code != strconv.Itoa(want) {
		t.Fatalf("result = %q\n%s", result, readSignalFile(t, log))
	}
	for _, text := range texts {
		if !strings.Contains(diagnostic, text) {
			t.Fatalf("result %q does not name %q\n%s", result, text, readSignalFile(t, log))
		}
	}
}

// requireHooks requires the hooks directory to hold exactly these hooks,
// samples aside.
func requireHooks(t *testing.T, hooks string, want map[string]string) {
	t.Helper()
	entries, err := os.ReadDir(hooks)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sample") {
			got[entry.Name()] = readSignalFile(t, filepath.Join(hooks, entry.Name()))
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("hooks = %q, want %q", got, want)
	}
}

// setupSignalReceipt answers the receipt of the repository under home, or
// nothing when there is none.
func setupSignalReceipt(t *testing.T, root, home string) string {
	t.Helper()
	dirs, err := signalBinding(ladderApp(t).bind(), home).dirs()
	if err != nil {
		t.Fatal(err)
	}
	_, identity, err := install.ProjectIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(setup.ReceiptPath(dirs.State, identity))
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeSignalFile(t *testing.T, path, text string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), mode); err != nil {
		t.Fatal(err)
	}
}
