package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

const (
	checkModule = "example.test/fx"
	checkBase   = "package fx\n\n// Add adds.\nfunc Add(a, b int) int { return a + b }\n"
	// checkWrong subtracts and drops an error: one red test, one lint finding.
	checkWrong = "package fx\n\nimport \"os\"\n\n// Clean removes x.\nfunc Clean() {\n\tos.Remove(\"x\")\n}\n\n// Add adds.\nfunc Add(a, b int) int { return a - b }\n"
	// checkRight is the same change done right and covered.
	checkRight = "package fx\n\n// Add adds.\nfunc Add(a, b int) int {\n\treturn b + a\n}\n"
	// checkUncovered adds a function no test runs.
	checkUncovered = checkRight + "\n// Twice doubles.\nfunc Twice(a int) int {\n\treturn 2 * a\n}\n"
	checkTest      = "package fx\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif got := Add(1, 2); got != 3 {\n\t\tt.Fatalf(\"Add = %d, want 3\", got)\n\t}\n}\n"
	checkLintLine  = "fx.go:7:11: Error return value of `os.Remove` is not checked (errcheck)"
)

// checkFixture is a module whose base commit, on the branch base, holds
// Add, with an app bound to the real go and a directory of fake tools.
type checkFixture struct {
	ta    testApp
	repo  *gittest.Repo
	tools string
}

func checkPolicy(capabilities string) string {
	return "golaw: 1\nkit: {version: " + testVersion + "}\ncapabilities: [" + capabilities + "]\n"
}

func newCheckFixture(t *testing.T, capabilities string) checkFixture {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("the tests run under go: %v", err)
	}
	tools := t.TempDir()
	b, _ := testBinding(t)
	b.environ = func() []string { return withPath(gittest.Isolated(), tools, filepath.Dir(goBin)) }
	ta := newTestApp(t)
	ta.bind = func() binding { return b }
	ta.buildInfo = func() buildinfo.Info { return b.info }
	ta.discover, ta.load, ta.snapshot = b.discover, b.load, b.snapshot
	ta.registry = func() (*engine.Registry, error) { return checks.Registry() }
	repo := gittest.New(t)
	repo.Commit("feat: the base", map[string]string{
		"go.mod": "module " + checkModule + "\n\ngo 1.27\n", "golaw.yml": checkPolicy(capabilities), "fx.go": checkBase,
	})
	repo.Git("branch", "base")
	return checkFixture{ta: ta, repo: repo, tools: tools}
}

// withPath replaces the PATH of environ with the directories.
func withPath(environ []string, dirs ...string) []string {
	out := slices.DeleteFunc(slices.Clone(environ), func(entry string) bool {
		name, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(name, "PATH")
	})
	return append(out, "PATH="+strings.Join(dirs, string(os.PathListSeparator)))
}

// linter installs a fake golangci-lint whose fmt over the two changed
// files is clean and whose run answers stdout with code.
func (f checkFixture) linter(t *testing.T, stdout string, code int) {
	t.Helper()
	runnertest.Install(t, f.tools, linterTool, runnertest.Behavior{}) // warmed with a clean answer first
	runnertest.Install(t, f.tools, linterTool, runnertest.Behavior{
		Probe: []string{"fmt", "--diff", "fx.go", "fx_test.go"}, Stdout: stdout, Code: code,
	})
}

func (f checkFixture) run(t *testing.T, args ...string) (int, string) {
	t.Helper()
	code := f.ta.run(t.Context(), append([]string{"check", f.repo.Root}, args...))
	return code, f.ta.stdout.String() + f.ta.stderr.String()
}

// assertOutput fails the test unless the exit code is the one wanted and
// the output holds every wanted text and no lacking one.
func assertOutput(t *testing.T, code, wantCode int, text string, want, lack []string) {
	t.Helper()
	if code != wantCode {
		t.Fatalf("exit = %d, output:\n%s", code, text)
	}
	for _, line := range want {
		if !strings.Contains(text, line) {
			t.Errorf("output lacks %q:\n%s", line, text)
		}
	}
	for _, line := range lack {
		if strings.Contains(text, line) {
			t.Errorf("output holds %q:\n%s", line, text)
		}
	}
}

func TestCheckReportsOnlyWhatIsRed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		source     string
		lint       string
		lintCode   int
		code       int
		want, lack []string
	}{
		{"red-a-failing-test-and-a-lint-finding", checkWrong, checkLintLine + "\n", 1, exitFindings, []string{
			"golaw check: 8 gates, 3 green, 2 red, 3 skipped (base base)\n",
			"red lint:\n  " + checkLintLine + "\n",
			"red test:\n  TestAdd: fx_test.go:7: Add = -1, want 3\n",
			"skipped coverage: the tests are red\n",
			"skipped migrations: the db capability is off\n",
		}, []string{"green vet", "red format", "red vet", "=== RUN"}},
		{"red-new-code-no-test-covers", checkUncovered, "", 0, exitFindings, []string{
			"golaw check: 8 gates, 5 green, 1 red, 2 skipped (base base)\n", "red coverage:\n", "fx.go: 2/4 covered, uncovered lines 10, 11",
		}, nil},
		{"near-the-change-done-right", checkRight, "", 0, exitClean, []string{
			"golaw check: 8 gates, 6 green, 0 red, 2 skipped (base base)\n",
		}, []string{"red "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCheckFixture(t, "go")
			f.linter(t, tc.lint, tc.lintCode)
			f.repo.Write("fx.go", tc.source)
			f.repo.Write("fx_test.go", checkTest)
			code, text := f.run(t, "--base", "base")
			assertOutput(t, code, tc.code, text, tc.want, tc.lack)
			calls := runnertest.Calls(t, f.tools, linterTool)
			i := slices.IndexFunc(calls, func(call []string) bool { return len(call) > 0 && call[0] == "run" })
			if i < 0 || !slices.Contains(calls[i], "--new-from-merge-base=base") || calls[i][len(calls[i])-1] != "." {
				t.Errorf("lint calls = %q", calls)
			}
		})
	}
}

func TestCheckBasesAndTools(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		args   []string
		remote bool
		linter bool
		code   int
		want   []string
	}{
		{"red-a-named-base-that-names-no-commit", []string{"--base", "nowhere"}, false, true, exitFailure, []string{"names no commit"}},
		{"near-the-remote-default-branch", nil, true, true, exitClean, []string{"(base refs/remotes/origin/main)"}},
		{"red-no-base-judges-the-whole-tree", nil, false, true, exitFindings, []string{
			"(whole tree)\nno remote default branch names a base here", "skipped coverage: no base names the new code", "red scan:",
		}},
		{"near-a-missing-linter-skips-its-gates", []string{"--base", "base"}, false, false, exitClean, []string{
			"skipped format: golangci-lint is not on PATH", "skipped lint: golangci-lint is not on PATH",
		}},
		{"near-verbose-names-the-green-gates", []string{"--base", "base", "--verbose"}, false, true, exitClean, []string{"green vet\n", "green test\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newCheckFixture(t, "go")
			if tc.linter {
				f.linter(t, "", 0)
			}
			if tc.remote {
				f.repo.Git("update-ref", "refs/remotes/origin/main", "base")
			}
			f.repo.Write("fx.go", checkRight)
			f.repo.Write("fx_test.go", checkTest)
			code, text := f.run(t, tc.args...)
			assertOutput(t, code, tc.code, text, tc.want, nil)
		})
	}
}

func TestCheckWritesTheJSONReport(t *testing.T) {
	t.Parallel()
	f := newCheckFixture(t, "go")
	f.linter(t, checkLintLine+"\n", 1)
	f.repo.Write("fx.go", checkWrong)
	f.repo.Write("fx_test.go", checkTest)
	code, _ := f.run(t, "--base", "base", "--json")
	var doc struct {
		Base  string `json:"base"`
		Gates []struct {
			Gate     string   `json:"gate"`
			Status   string   `json:"status"`
			Failures []string `json:"failures"`
		} `json:"gates"`
	}
	if err := json.Unmarshal(f.ta.stdout.Bytes(), &doc); err != nil || code != exitFindings || doc.Base != "base" {
		t.Fatalf("exit = %d, %v:\n%s", code, err, f.ta.stdout.String())
	}
	if doc.Gates[2].Gate != gateLint || doc.Gates[2].Status != "red" || !slices.Equal(doc.Gates[2].Failures, []string{checkLintLine}) {
		t.Errorf("lint = %+v", doc.Gates[2])
	}
}
