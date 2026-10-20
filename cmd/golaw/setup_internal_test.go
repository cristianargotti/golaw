package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/setup"
)

func TestSetupRefusesPinMismatchBeforeInstallation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, version string
	}{
		{"older pin", "9.9.8"},
		{"newer pin", "9.9.10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := gittest.New(t)
			data := strings.Replace(string(renderedPolicy(t)), testVersion, tc.version, 1)
			repo.Commit("chore: establish policy", map[string]string{
				"golaw.yml": data,
				"go.mod":    "module example.test/service\n\ngo 1.27\n",
			})
			ta := bindApp(t)
			if code := ta.run(t.Context(), []string{"setup", repo.Root}); code != exitFailure {
				t.Fatalf("exit = %d", code)
			}
			text := ta.stderr.String()
			if !strings.Contains(text, tc.version) || strings.Contains(text, "unknown command") {
				t.Fatalf("pin diagnosis = %q", text)
			}
		})
	}
}

func TestSetupUninstallWithoutReceiptIsSilent(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: establish policy", map[string]string{
		"golaw.yml": string(renderedPolicy(t)),
		"go.mod":    "module example.test/service\n\ngo 1.27\n",
	})
	ta := bindApp(t)
	if code := ta.run(t.Context(), []string{"setup", "--uninstall", repo.Root}); code != exitClean {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
	if ta.stdout.Len() != 0 {
		t.Fatalf("empty uninstall output = %q", ta.stdout.String())
	}
}

// fakeGo puts first on PATH a go that answers every command with the
// output of go env GOBIN GOPATH.
func fakeGo(t *testing.T, stdout string) {
	t.Helper()
	dir := t.TempDir()
	runnertest.Install(t, dir, "go", runnertest.Behavior{Probe: []string{"version"}, Stdout: stdout})
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

//nolint:paralleltest // the fake go is found through PATH
func TestSetupOptionsResolveTheBinDirectory(t *testing.T) {
	gopath, other, set := t.TempDir(), t.TempDir(), t.TempDir()
	for _, tc := range []struct{ name, stdout, want string }{
		{"GOBIN set", set + "\n" + gopath + "\n", set},
		{"GOBIN empty", "\n" + gopath + "\n", filepath.Join(gopath, "bin")},
		{"two GOPATH entries", "\n" + gopath + string(os.PathListSeparator) + other + "\n", filepath.Join(gopath, "bin")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeGo(t, tc.stdout)
			repo := gittest.New(t)
			repo.Commit("chore: establish policy", map[string]string{
				"golaw.yml": string(renderedPolicy(t)),
				"go.mod":    "module example.test/service\n\ngo 1.27\n",
			})
			options, err := bindApp(t).bind().setupOptions(t.Context(), repo.Root, now())
			if err != nil || options.BinDir != tc.want {
				t.Fatalf("bin directory = %q, %v, want %q", options.BinDir, err, tc.want)
			}
		})
	}
}

// Setup stays strict before the adoption commit, and its refusal names the
// fix: the person commits the golaw.yml init --apply wrote, then arms.
func TestSetupAsksForTheAdoptionCommitFirst(t *testing.T) {
	t.Parallel()
	ta := bindApp(t)
	root := repository(t, false)
	gitRun(t, root, "add", "go.mod")
	gitRun(t, root, "commit", "-q", "-m", "build: start the module")
	b := ta.bind()
	data := renderedPolicy(t)
	p, err := b.compile(data, now())
	if err != nil {
		t.Fatal(err)
	}
	if err := b.enroll(root, data, p, now()); err != nil {
		t.Fatal(err)
	}
	if code := ta.run(t.Context(), []string{"setup", root}); code != exitFailure || !strings.Contains(ta.stderr.String(), "commit golaw.yml, then run golaw setup") {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
}

// The command reaches the install plan with the real go: without the
// lefthook.yml init writes, the plan names the init it needs.
func TestSetupNamesTheInitItNeeds(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	repo.Commit("chore: establish policy", map[string]string{
		"golaw.yml": string(renderedPolicy(t)),
		"go.mod":    "module example.test/service\n\ngo 1.27\n",
	})
	ta := bindApp(t)
	if code := ta.run(t.Context(), []string{"setup", repo.Root}); code != exitFailure || !strings.Contains(ta.stderr.String(), "golaw init --apply") {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
}

// The command applies the plan through the binding: with golaw and every
// pinned tool verified on PATH, a lefthook install that fails names
// lefthook and leaves no receipt (review of the plan, point C22). A run
// that writes the receipt needs a lefthook that writes the hooks, which a
// fake does not; the package tests of SET-2 and the setup rows of ACC-3
// prove it.
func TestSetupAppliesThroughTheBinding(t *testing.T) {
	bin := t.TempDir()
	for _, tool := range runner.Toolchain() {
		runnertest.Install(t, bin, tool.Name, runnertest.Behavior{
			Probe: tool.Probe, Version: tool.Version,
			Stderr: tool.Name + " failed\n", Code: 1,
		})
	}
	runnertest.Install(t, bin, "golaw", runnertest.Behavior{Probe: []string{"version"}})
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	repo := gittest.New(t)
	repo.Commit("chore: establish policy", map[string]string{
		"golaw.yml": string(renderedPolicy(t)),
		"go.mod":    "module example.test/service\n\ngo 1.27\n",
		"lefthook.yml": "pre-commit:\n  commands:\n    golaw:\n      run: golaw hook pre-commit\n" +
			"commit-msg:\n  commands:\n    golaw:\n      run: golaw hook commit-msg '{1}'\n" +
			"pre-push:\n  commands:\n    golaw:\n      run: golaw hook pre-push '{1}' '{2}'\n",
	})
	ta := bindApp(t)
	if code := ta.run(t.Context(), []string{"setup", repo.Root}); code != exitFailure ||
		!strings.Contains(ta.stderr.String(), "lefthook install exited 1") {
		t.Fatalf("exit = %d, stderr = %q", code, ta.stderr.String())
	}
	dirs, err := ta.bind().dirs()
	if err != nil {
		t.Fatal(err)
	}
	_, identity, err := install.ProjectIdentity(repo.Root)
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(setup.ReceiptPath(dirs.State, identity)); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("receipt after a failed hook installation: %v", statErr)
	}
}
