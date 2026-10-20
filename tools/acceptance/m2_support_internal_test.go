package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
)

// m2Version and m2Executable are the version and the bytes of the
// candidate every simulated row runs.
const (
	m2Version    = "0.2.0"
	m2Executable = "simulated M2 executable\n"
)

// m2Script is one simulated row: Execute answers every command the row
// runs, Calls counts every command it received, and Targets counts the
// commands that carry the row's assertion, the operation the row judges.
type m2Script struct {
	Execute executor
	Calls   int
	Targets int
}

// m2Reply is the answer to one simulated command; target marks a command
// that carries the row's assertion.
type m2Reply struct {
	got    result
	target bool
}

// m2Answer answers one command of a simulated row, or reports false to
// leave the command to the fixture executor.
type m2Answer func(ctx context.Context, c command) (m2Reply, bool)

// m2Moment is the fixed time every simulated record carries, so a record
// written twice keeps its bytes.
func m2Moment() time.Time { return time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC) }

// m2TestSuite is a simulated suite: the real Git, a Go path that is never
// started, the candidate's bytes at its path and a shared state whose tools
// count as verified. It builds, installs and starts nothing.
func m2TestSuite(t *testing.T) *suite {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	simulated := filepath.Join(base, "simulated")
	lookup := func(name string) (string, error) {
		if name == "git" {
			return git, nil
		}
		return filepath.Join(simulated, executable(name)), nil
	}
	s := newSuite(testRoot(t), base, nil, lookup)
	s.git, s.goTool = git, filepath.Join(simulated, executable("go"))
	s.m2 = &m2Shared{Base: s.base, Bin: filepath.Join(s.base, "tools"), Ready: true}
	s.m2OS = runtime.GOOS
	s.env = m2Environment(nil, s.home, filepath.Dir(s.binary), s.base)
	m2Write(t, s.binary, m2Executable)
	s.digest = install.Digest([]byte(m2Executable))
	for _, name := range []string{"go", "sh"} {
		m2Write(t, filepath.Join(simulated, executable(name)), "simulated "+name+"\n")
	}
	for _, tool := range runner.Toolchain() {
		m2Write(t, filepath.Join(s.m2.Bin, executable(tool.Name)), "simulated "+tool.Name+"\n")
	}
	return s
}

// m2FixtureExecutor runs the real Git, simulates a Go build and the
// candidate's init --apply, and refuses every other command, so an
// accidental real execution fails its row instead of passing silently.
func m2FixtureExecutor(t *testing.T, s *suite) executor {
	t.Helper()
	return func(ctx context.Context, c command) result {
		switch {
		case c.program == s.git:
			return runCommand(ctx, c)
		case c.program == s.goTool && len(c.args) > 0 && c.args[0] == goBuild:
			return simulatedBuild(t, c)
		case c.program == s.binary && slices.Equal(c.args, []string{commandInit, flagApply}):
			return m2Init(t, s, c)
		}
		return result{err: ErrAcceptance}
	}
}

// newM2Script counts every command of a row and sends each one the answer
// does not take to the fixture executor.
func newM2Script(t *testing.T, s *suite, answer m2Answer) *m2Script {
	t.Helper()
	fixture := m2FixtureExecutor(t, s)
	script := &m2Script{}
	script.Execute = func(ctx context.Context, c command) result {
		script.Calls++
		reply, handled := answer(ctx, c)
		if !handled {
			return fixture(ctx, c)
		}
		if reply.target {
			script.Targets++
		}
		return reply.got
	}
	return script
}

// m2Init stands in for the candidate's init --apply: the bootstrap policy
// and the core templates rendered into the repository, each written only
// when it is absent, and the enrollment record of the policy it leaves.
func m2Init(t *testing.T, s *suite, c command) result {
	t.Helper()
	data, err := bootstrap.Render(bootstrap.Inputs{
		KitVersion: m2Version, Language: "pt-BR", Capabilities: []string{"go"},
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	})
	if err != nil {
		return result{code: 2, stderr: err.Error()}
	}
	files := m2Render(t, c.dir)
	for i := range files {
		m2WriteOnce(t, filepath.Join(c.dir, filepath.FromSlash(files[i].Path)), files[i].Data)
	}
	m2WriteOnce(t, filepath.Join(c.dir, policyFile), data)
	m2Enroll(t, s, c.dir, c.env)
	return result{}
}

// m2Render renders the core templates for the module a repository's go.mod
// names, at the candidate's version and the pins of the toolchain.
func m2Render(t *testing.T, dir string) []assets.Rendered {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, moduleFile))
	if err != nil {
		t.Fatal(err)
	}
	values := assets.Values{
		KitVersion: m2Version, Language: "pt-BR", Capabilities: []string{"go"},
		Decisions: "docs/adr", Main: bootstrap.MainRoot, Tools: map[string]string{},
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if module, found := strings.CutPrefix(line, "module "); found {
			values.ModulePath = module
		}
		if version, found := strings.CutPrefix(line, "go "); found {
			values.GoVersion = version
		}
	}
	for _, tool := range runner.Toolchain() {
		values.Tools[tool.Name] = tool.Version
	}
	files, err := assets.RenderAll(golaw.FS, values)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// m2WriteOnce writes a generated file only when it is absent: init never
// replaces a file the repository holds.
func m2WriteOnce(t *testing.T, path string, data []byte) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		return
	}
	m2Write(t, path, string(data))
}

// m2Write writes a simulated file with its directories; the simulation
// writes only beneath the temporary directories of its test.
func m2Write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), execMode); err != nil {
		t.Fatal(err)
	}
}

// m2Enroll writes the enrollment record of the golaw.yml the checkout of a
// repository holds, at that file's pin and the fixed moment.
func m2Enroll(t *testing.T, s *suite, dir string, env []string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, policyFile))
	if err != nil {
		t.Fatal(err)
	}
	file, err := policy.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	root, _, err := install.ProjectIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	record := policy.Record{
		Root: root, PolicyDigest: policy.Digest(file), KitVersion: file.Kit.Version,
		Bootstrap: string(data), WrittenBy: policy.WrittenByInit, WrittenAt: m2Moment(),
	}
	if err := policy.WriteRecord(m2Dirs(t, s, env).State, record); err != nil {
		t.Fatal(err)
	}
}

// m2Dirs is the state and configuration directories the environment of a
// command selects on the suite's operating system.
func m2Dirs(t *testing.T, s *suite, env []string) policy.Dirs {
	t.Helper()
	dirs, err := policy.DefaultDirs(s.m2OS, func(name string) string {
		return environmentValue(env, name)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dirs
}
