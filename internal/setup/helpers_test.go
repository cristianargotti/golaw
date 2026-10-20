package setup_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/setup"
)

const (
	fakeHook    = "owned hook\n"
	foreignHook = "foreign hook\n"
	// routedLefthook is the lefthook.yml init writes, which routes each
	// rung to golaw.
	routedLefthook = "min_version: 2.1.0\n" +
		"pre-commit:\n  commands:\n    golaw:\n      run: golaw hook pre-commit\n" +
		"commit-msg:\n  commands:\n    golaw:\n      run: golaw hook commit-msg '{1}'\n" +
		"pre-push:\n  commands:\n    golaw:\n      run: golaw hook pre-push '{1}' '{2}'\n      use_stdin: true\n"
)

// errUnexpected is the answer of the fake toolchain to a command it does
// not play.
var errUnexpected = errors.New("unexpected fake command")

type setupProbe struct {
	ready    map[string]bool
	calls    []runner.Command
	failTool bool
	failHook bool
	failUndo bool
	partial  bool
	shadow   bool
	old      bool
	hooks    string
	checksum string
	golaw    string
	repo     *gittest.Repo
	t        *testing.T
	bin      string
	git      string
	root     string
}

func setupFixture(t *testing.T) (setup.Options, setup.Dependencies, *setupProbe) {
	t.Helper()
	repo := gittest.New(t)
	repo.Write("lefthook.yml", routedLefthook)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "tools área")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	probe := &setupProbe{
		ready: make(map[string]bool), repo: repo,
		hooks:    filepath.Join(repo.GitDir(), "hooks"),
		checksum: filepath.Join(repo.GitDir(), "info", "lefthook.checksum"),
		t:        t, bin: bin, git: git, root: repo.Root,
	}
	probe.golaw = runnertest.Install(t, bin, "golaw", runnertest.Behavior{})
	t.Setenv("PATH", bin+string(os.PathListSeparator)+filepath.Dir(git))
	options := setup.Options{
		Root: repo.Root, StateDir: t.TempDir(), Version: "0.2.0",
		BinDir: bin, Env: gittest.Isolated(),
		Now: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
	}
	deps := setup.Dependencies{Verify: probe.verify, Run: probe.run}
	return options, deps, probe
}

// verify answers a tool of the fake toolchain: a shadowed lefthook and an
// old tool at another version, a tool not installed missing, and a ready
// tool in the bin directory.
func (p *setupProbe) verify(_ context.Context, tool runner.Tool, _ []string) (string, error) {
	if p.shadow && tool.Name == "lefthook" {
		return "", runner.ErrToolVersion
	}
	if !p.ready[tool.Name] {
		if p.old {
			return "", runner.ErrToolVersion
		}
		return "", runner.ErrToolMissing
	}
	name := tool.Name
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(p.bin, name), nil
}

// run plays every command setup starts: the real git in the repository,
// and the fake go, lefthook and version probes.
func (p *setupProbe) run(ctx context.Context, command runner.Command) (runner.Output, error) {
	p.calls = append(p.calls, command)
	if command.Dir != p.root {
		p.t.Errorf("command directory = %q", command.Dir)
	}
	name := strings.TrimSuffix(filepath.Base(command.Name), ".exe")
	if name == "git" {
		command.Name = p.git
		out, err := runner.Run(ctx, command)
		if err != nil {
			return out, fmt.Errorf("git: %w", err)
		}
		return out, nil
	}
	if err := ctx.Err(); err != nil {
		return runner.Output{}, err
	}
	if out, handled := p.play(name, command); handled {
		return out, nil
	}
	p.t.Errorf("unexpected command: %s %v", command.Name, command.Args)
	return runner.Output{}, errUnexpected
}

// play answers go install, lefthook install and uninstall, and the version
// probe of each pinned tool.
func (p *setupProbe) play(name string, command runner.Command) (runner.Output, bool) {
	switch {
	case name == "go":
		return p.install(command)
	case name == "lefthook" && len(command.Args) == 1 &&
		(command.Args[0] == "install" || command.Args[0] == "uninstall"):
		return p.hooksCommand(p.t, command.Args[0]), true
	}
	for _, tool := range runner.Toolchain() {
		if name == tool.Name && slices.Equal(command.Args, tool.Probe) {
			if p.ready[name] {
				return runner.Output{Stdout: []byte(tool.Version + "\n")}, true
			}
			return runner.Output{Code: 1}, true
		}
	}
	return runner.Output{}, false
}

// install plays go install: a failure when failTool is set, and otherwise
// the tool whose install arguments it carries made ready.
func (p *setupProbe) install(command runner.Command) (runner.Output, bool) {
	if p.failTool {
		return runner.Output{Code: 1, Stderr: []byte("installation failed\n")}, true
	}
	if !slices.Contains(command.Env, "GOBIN="+p.bin) {
		p.t.Errorf("installation lacks GOBIN: %v", command.Env)
	}
	for _, tool := range runner.Toolchain() {
		if slices.Equal(command.Args, tool.InstallArgs()) {
			p.ready[tool.Name] = true
			return runner.Output{}, true
		}
	}
	return runner.Output{}, false
}

func setupWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o700); err != nil {
		t.Fatal(err)
	}
}
