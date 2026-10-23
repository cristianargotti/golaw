package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestM3ExpandEvaluatesOnlyTheKnownExpressions(t *testing.T) {
	t.Parallel()
	vars := map[string]string{"vars.GOLAW_ENROLLED": "", "github.workspace": "/work"}
	for _, tc := range []struct {
		name, text, want string
		fail             bool
	}{
		{"near plain text", "make ci", "make ci", false},
		{"near a known variable", "cd ${{ github.workspace }} && make ci", "cd /work && make ci", false},
		{"near an empty repository variable", "GOLAW_ENROLLED=${{vars.GOLAW_ENROLLED}}", "GOLAW_ENROLLED=", false},
		{"red an expression the replay does not evaluate", "echo ${{ secrets.TOKEN }}", "", true},
	} {
		got, err := m3Expand(tc.text, vars)
		if (err != nil) != tc.fail || (!tc.fail && got != tc.want) {
			t.Fatalf("%s: %q, %v", tc.name, got, err)
		}
		if tc.fail && !errors.Is(err, ErrAcceptance) {
			t.Fatalf("%s: %v is no acceptance failure", tc.name, err)
		}
	}
}

func TestM3ApplyRunnerFilesCarriesVariablesAndPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := m3RunnerFiles{env: filepath.Join(dir, "env"), path: filepath.Join(dir, "path")}
	checkedWrite(t, files.env, "GOTOOLCHAIN=auto\n")
	checkedWrite(t, files.path, "/opt/tool/bin\n")
	env, err := m3ApplyRunnerFiles([]string{"GOTOOLCHAIN=local", "PATH=/bin"}, files)
	if err != nil {
		t.Fatal(err)
	}
	if environmentValue(env, "GOTOOLCHAIN") != "auto" ||
		!strings.HasPrefix(environmentValue(env, "PATH"), "/opt/tool/bin"+string(os.PathListSeparator)) {
		t.Fatalf("env = %q", env)
	}
	for _, path := range []string{files.env, files.path} {
		if data, err := os.ReadFile(path); err != nil || len(data) != 0 {
			t.Fatalf("%s kept %q, %v", path, data, err)
		}
	}
	checkedWrite(t, files.env, "NOTES<<EOF\nline\nEOF\n")
	if _, err := m3ApplyRunnerFiles(env, files); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("a multiline value = %v", err)
	}
}

func TestM3ReplayRunsEveryStepInOrder(t *testing.T) {
	t.Parallel()
	steps := []assets.RunStep{
		{Job: "ci", Name: "Use the toolchain named in go.mod", Run: `echo "GOTOOLCHAIN=auto" >> "$GITHUB_ENV"`},
		{Job: "ci", Name: "Run the pipeline", Run: "make ci", Env: map[string]string{"GOLAW_ENROLLED": "${{ vars.GOLAW_ENROLLED }}"}},
		{Job: "ci", Name: "Reject drift", Run: "git diff --exit-code"},
	}
	for _, tc := range []struct {
		name    string
		failing string
		fail    bool
	}{
		{"near every step passes", "", false},
		{"red the pipeline fails", "make ci", true},
	} {
		s := m2TestSuite(t)
		var scripts []string
		s.execute = m3ScriptRecorder(t, &scripts, tc.failing)
		err := s.m3Replay(t.Context(), t.TempDir(), steps)
		if (err != nil) != tc.fail {
			t.Fatalf("%s: replay = %v", tc.name, err)
		}
		if tc.fail && (!strings.Contains(err.Error(), "Run the pipeline") || len(scripts) != 2) {
			t.Fatalf("%s: %v after %q", tc.name, err, scripts)
		}
		if !tc.fail && !slices.Equal(scripts, []string{steps[0].Run, "make ci", "git diff --exit-code"}) {
			t.Fatalf("%s: scripts = %q", tc.name, scripts)
		}
	}
}

// m3ScriptRecorder answers every step of a replay, records its script,
// requires the runner variables on each and fails the one script named.
func m3ScriptRecorder(t *testing.T, scripts *[]string, failing string) executor {
	t.Helper()
	return func(_ context.Context, c command) result {
		script := c.args[len(c.args)-1]
		*scripts = append(*scripts, script)
		if environmentValue(c.env, "GITHUB_ENV") == "" || environmentValue(c.env, "CI") != "true" {
			t.Fatalf("a step ran without the runner variables: %q", c.env)
		}
		if script == failing {
			return result{code: 2}
		}
		return result{}
	}
}

func TestM3TemplateReplaysTheRenderedCI(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	var calls [][]string
	service := m3ServiceAnswer(t, s, &calls)
	var scripts []string
	s.execute = newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
		if filepath.Base(c.program) == executable("bash") {
			scripts = append(scripts, c.args[len(c.args)-1])
			return m2Reply{target: true}, true
		}
		return service(ctx, c)
	}).Execute
	if err := s.templateM3(t.Context(), "go"); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(scripts, func(script string) bool { return strings.Contains(script, "make ci") }) {
		t.Fatalf("the generated CI ran no make ci: %q", scripts)
	}
	if err := s.templateM3(t.Context(), "absent"); err == nil {
		t.Fatal("an unknown combination must fail")
	}
	if !slices.Equal(m3TemplateNames(), []string{"every", "go", "go-api", "go-db", "go-hexagonal", "go-image"}) {
		t.Fatalf("combinations = %q", m3TemplateNames())
	}
}
