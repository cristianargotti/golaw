package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

// m3InitWriting answers init by writing one CI file, or none when body is
// empty, so a test reaches each refusal of templateM3 after the adoption.
func m3InitWriting(t *testing.T, s *suite, body string) m2Answer {
	t.Helper()
	return func(_ context.Context, c command) (m2Reply, bool) {
		if c.program != s.binary {
			return m2Reply{}, false
		}
		if body != "" {
			m2Write(t, filepath.Join(c.dir, filepath.FromSlash(m3CIFile)), body)
		}
		return m2Reply{target: true}, true
	}
}

func TestM3TemplateNamesWhatItCannotReplay(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body, want string }{
		{"red no CI file", "", "read acceptance file"},
		{"red an unreadable CI file", "jobs: [\n", "read the run steps"},
		{"red only action steps", "jobs:\n  ci:\n    steps:\n      - uses: actions/checkout@v5\n", "holds no run step"},
	} {
		s := m2TestSuite(t)
		s.execute = newM2Script(t, s, m3InitWriting(t, s, tc.body)).Execute
		if err := s.templateM3(t.Context(), "go"); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
	s := m2TestSuite(t)
	checkedWrite(t, filepath.Join(s.base, dirRepos), "a file where the repositories go\n")
	if err := s.templateM3(t.Context(), "go"); err == nil {
		t.Fatal("a replay over a failed adoption must fail")
	}
}

func TestM3ReplayNamesTheStepThatCannotRun(t *testing.T) {
	t.Parallel()
	unnamed := assets.RunStep{Job: "ci", Run: "make ci\nmake vet"}
	for _, tc := range []struct {
		name string
		step assets.RunStep
		got  result
		want string
	}{
		{"red an unknown expression in the command", assets.RunStep{Job: "ci", Run: "echo ${{ secrets.TOKEN }}"}, result{}, "secrets.TOKEN"},
		{"red an unknown expression in the env", assets.RunStep{Job: "ci", Run: "true", Env: map[string]string{"A": "${{ github.token }}"}}, result{}, "env A"},
		{"red bash not started", unnamed, result{err: ErrAcceptance}, `step "Run make ci" of job ci: execute bash`},
		{"red bash fails", unnamed, result{code: 1, stdout: "lint failed\n"}, "lint failed"},
	} {
		s := m2TestSuite(t)
		s.execute = func(context.Context, command) result { return tc.got }
		if err := s.m3Replay(t.Context(), t.TempDir(), []assets.RunStep{tc.step}); !errors.Is(err, ErrAcceptance) ||
			!strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}

func TestM3ReplayNeedsItsRunner(t *testing.T) {
	t.Parallel()
	steps := []assets.RunStep{{Job: "ci", Name: "Run the pipeline", Run: "make ci"}}
	s := m2TestSuite(t)
	s.lookup = func(string) (string, error) { return "", ErrAcceptance }
	if err := s.m3Replay(t.Context(), t.TempDir(), steps); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("a machine without bash = %v", err)
	}
	for _, blocked := range []string{"runner", "runner-temp"} {
		scope := m2TestSuite(t)
		checkedWrite(t, filepath.Join(scope.base, blocked), "a file where the runner writes\n")
		if err := scope.m3Replay(t.Context(), t.TempDir(), steps); err == nil {
			t.Fatalf("a runner over the file %s must fail", blocked)
		}
	}
	s = m2TestSuite(t)
	s.execute = func(_ context.Context, c command) result {
		checkedWrite(t, environmentValue(c.env, "GITHUB_ENV"), "NOTES<<EOF\nline\nEOF\n")
		return result{}
	}
	if err := s.m3Replay(t.Context(), t.TempDir(), steps); !errors.Is(err, ErrAcceptance) ||
		!strings.Contains(err.Error(), `after step "Run the pipeline"`) {
		t.Fatalf("a step that writes a multiline value = %v", err)
	}
}

func TestM3ReplayCarriesTheRunnerFilesAndTheCacheFlag(t *testing.T) {
	t.Parallel()
	steps := []assets.RunStep{
		{Job: "ci", Run: "write", Env: map[string]string{"GOFLAGS": "-mod=readonly"}},
		{Job: "ci", Run: "read", Env: map[string]string{"GOFLAGS": "-modcacherw -mod=readonly", "ROOT": "${{ github.workspace }}"}},
	}
	s := m2TestSuite(t)
	repo := t.TempDir()
	var seen [][]string
	s.execute = func(_ context.Context, c command) result {
		seen = append(seen, c.env)
		if c.args[len(c.args)-1] == "write" {
			checkedWrite(t, environmentValue(c.env, "GITHUB_ENV"), "\r\nSTAGE=second\r\n")
			checkedWrite(t, environmentValue(c.env, "GITHUB_PATH"), "/opt/one\n/opt/two\n")
		}
		if c.dir != repo {
			t.Fatalf("a step ran in %s", c.dir)
		}
		return result{}
	}
	if err := s.m3Replay(t.Context(), repo, steps); err != nil {
		t.Fatal(err)
	}
	separator := string(os.PathListSeparator)
	if environmentValue(seen[0], "GOFLAGS") != "-mod=readonly -modcacherw" ||
		environmentValue(seen[1], "GOFLAGS") != "-modcacherw -mod=readonly" ||
		environmentValue(seen[1], "STAGE") != "second" || environmentValue(seen[1], "ROOT") != repo ||
		!strings.HasPrefix(environmentValue(seen[1], "PATH"), "/opt/two"+separator+"/opt/one"+separator) ||
		environmentValue(seen[0], "NPM_CONFIG_PREFIX") == "" {
		t.Fatalf("environments = %q", seen)
	}
}

func TestM3ApplyRunnerFilesNeedsBothFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := m3RunnerFiles{env: filepath.Join(dir, "env"), path: filepath.Join(dir, "path")}
	if _, err := m3ApplyRunnerFiles(nil, files); err == nil {
		t.Fatal("an absent env file must fail")
	}
	checkedWrite(t, files.env, "")
	if _, err := m3ApplyRunnerFiles(nil, files); err == nil {
		t.Fatal("an absent path file must fail")
	}
	checkedWrite(t, files.path, "")
	for _, line := range []string{"NO_VALUE", "=value"} {
		checkedWrite(t, files.env, line+"\n")
		if _, err := m3ApplyRunnerFiles(nil, files); !errors.Is(err, ErrAcceptance) {
			t.Fatalf("the line %q = %v", line, err)
		}
	}
	if err := os.Remove(files.path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(files.path, dirMode); err != nil {
		t.Fatal(err)
	}
	checkedWrite(t, files.env, "A=b\n")
	if _, err := m3ApplyRunnerFiles(nil, files); err == nil {
		t.Fatal("a path file that is a directory must fail")
	}
}
