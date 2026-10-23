package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// m5Reply answers the commands of program whose arguments start with
// prefix, program empty meaning the candidate; a reply marked once answers
// one command and then steps aside for the next reply of the same prefix.
type m5Reply struct {
	program string
	prefix  []string
	once    bool
	answer  func(t *testing.T, c command) result
}

// m5Answer is a reply that answers got and changes nothing.
func m5Answer(got result) func(*testing.T, command) result {
	return func(*testing.T, command) result { return got }
}

// m5After is a reply that changes the files the command would change, then
// answers got.
func m5After(effect func(*testing.T, command), got result) func(*testing.T, command) result {
	return func(t *testing.T, c command) result {
		t.Helper()
		effect(t, c)
		return got
	}
}

// m5Script answers commands from replies, the first unused one whose
// program and prefix match, and sends every other command to the fixture
// executor of M2, which runs git for real and simulates a go build; a
// candidate command no reply names fails the test.
func m5Script(t *testing.T, s *suite, replies ...m5Reply) *m2Script {
	t.Helper()
	used := make([]bool, len(replies))
	return newM2Script(t, s, func(_ context.Context, c command) (m2Reply, bool) {
		for i, r := range replies {
			program := r.program
			if program == "" {
				program = s.binary
			}
			if used[i] || c.program != program || len(c.args) < len(r.prefix) || !slices.Equal(c.args[:len(r.prefix)], r.prefix) {
				continue
			}
			used[i] = r.once
			return m2Reply{got: r.answer(t, c), target: true}, true
		}
		if c.program != s.binary {
			return m2Reply{}, false
		}
		t.Errorf("unexpected candidate command %q", c.args)
		return m2Reply{got: result{err: ErrAcceptance}, target: true}, true
	})
}

// m5TestDirs are the golaw directories a command's environment resolves.
func m5TestDirs(t *testing.T, env []string) policy.Dirs {
	t.Helper()
	dirs, err := policy.DefaultDirs(runtime.GOOS, func(key string) string { return environmentValue(env, key) })
	if err != nil {
		t.Fatal(err)
	}
	return dirs
}

// m5FakeInstall stands in for profile install: the bundle of the version
// copied into the current directory of the command's home.
func m5FakeInstall(version string) func(*testing.T, command) {
	return func(t *testing.T, c command) {
		t.Helper()
		dir := profile.Dir(m5TestDirs(t, c.env).Config, m5Name)
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
		if err := m5CopyBundle(testRoot(t), dir); err != nil {
			t.Fatal(err)
		}
		if version == m5NewerVersion {
			if err := m5NewerBundle(dir); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// m5FakeInit stands in for init --apply: the policy rendered at the
// candidate's version, pinned to the current stamp when the command names
// the profile flag, written only when the checkout holds none.
func m5FakeInit(t *testing.T, c command) {
	t.Helper()
	in := bootstrap.Inputs{
		KitVersion: m5CandidateVersion, Language: "pt-BR", Capabilities: []string{"go", "image"},
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	}
	if slices.Contains(c.args, m5ProfileFlag) {
		stamp, err := policy.LoadStamp(policy.StampPath(m5TestDirs(t, c.env).Config, m5Name))
		if err != nil {
			t.Fatal(err)
		}
		in.Profile = &policy.ProfileRef{Name: stamp.Name, Source: stamp.Source, Version: stamp.Version, Digest: stamp.Digest}
	}
	data, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(c.dir, policyFile)
	if _, err := os.Stat(path); err == nil {
		return
	}
	m2Write(t, path, string(data))
}

// m5DoctorStream is doctor --json as M4 prints it: one document per agent,
// the lines given in the second.
func m5DoctorStream(t *testing.T, lines ...doctor.Line) string {
	t.Helper()
	documents := make([]string, 0, 2)
	for _, report := range []doctor.Report{
		{Schema: 1, Lines: []doctor.Line{{ID: "doctor.agent", State: "ok", Code: "agent"}}},
		{Schema: 1, Lines: lines},
	} {
		body, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		documents = append(documents, string(body))
	}
	return strings.Join(documents, "\n") + "\n"
}

// m5Clean requires a repository with nothing to commit.
func m5Clean(t *testing.T, s *suite, repo string) {
	t.Helper()
	status := runCommand(t.Context(), command{program: s.git, args: []string{"status", "--porcelain"}, dir: repo, env: s.env})
	if status.err != nil || status.code != 0 || status.stdout != "" {
		t.Fatalf("the repository is not committed: %+v", status)
	}
}

// m5Sweep runs a case once with replies, counting its commands, then once
// per command with that command failing, and requires every such run to
// fail: no step of a row swallows the failure of a command it ran.
func m5Sweep(t *testing.T, category string, c m5Case, replies func(t *testing.T, s *suite) []m5Reply) {
	t.Helper()
	s := m2TestSuite(t)
	scope := s.m5Scope(category, c.name)
	script := m5Script(t, scope, replies(t, scope)...)
	scope.execute = script.Execute
	if err := c.run(scope, t.Context()); err != nil {
		t.Fatalf("%s: %v", c.name, err)
	}
	for k := 1; k <= script.Calls; k++ {
		failing := s.m5Scope(category, c.name+"-fail-"+strconv.Itoa(k))
		inner := m5Script(t, failing, replies(t, failing)...)
		calls := 0
		failing.execute = func(ctx context.Context, cmd command) result {
			calls++
			if calls == k {
				return result{code: 2, stderr: "golaw: injected failure\n"}
			}
			return inner.Execute(ctx, cmd)
		}
		if err := c.run(failing, t.Context()); err == nil {
			t.Errorf("%s passed with its command %d failing", c.name, k)
		}
	}
}
