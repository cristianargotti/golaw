package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

func TestM6JudgeSameComparesTheTwinPathsOnly(t *testing.T) {
	t.Parallel()
	twin, err := surfacefix.Files(surfacefix.Red)
	if err != nil {
		t.Fatal(err)
	}
	found := m6Found(surfacefix.Expectations(), true)
	onConfig := append(append([]model.Finding{}, found...), model.Finding{Check: "lint.yamllint", Path: ".hadolint.yaml", Code: "document-start", Line: 1, Severity: model.SeverityFail})
	rows := []struct {
		name              string
		plain, configured result
		pass              bool
	}{
		{"near the same findings", result{code: 1, stdout: m6Doc(t, found)}, result{code: 1, stdout: m6Doc(t, found)}, true},
		{"near a finding on the configuration file itself", result{code: 1, stdout: m6Doc(t, found)}, result{code: 1, stdout: m6Doc(t, onConfig)}, true},
		{"red a rule disabled", result{code: 1, stdout: m6Doc(t, found)}, result{code: 1, stdout: m6Doc(t, found[1:])}, false},
		{"red nothing to compare", result{stdout: m6Doc(t, nil)}, result{stdout: m6Doc(t, nil)}, false},
		{"red the configured run failed", result{code: 1, stdout: m6Doc(t, found)}, result{code: 2, stdout: m6Doc(t, nil)}, false},
	}
	for _, row := range rows {
		if err := m6JudgeSame(row.plain, row.configured, twin); (err == nil) != row.pass {
			t.Errorf("%s: %v", row.name, err)
		}
	}
}

func TestM6ConfigRowLintsBothRepositories(t *testing.T) {
	t.Parallel()
	s := m6TestSuite(t)
	var dirs []string
	found := m6Found(surfacefix.Expectations(), true)
	s.execute = m5Script(t, s.suite, m6InitReply(), m5Reply{prefix: []string{m6LintCommand, m6SurfaceFlag, m2JSONFlag}, answer: func(t *testing.T, c command) result {
		t.Helper()
		dirs = append(dirs, c.dir)
		return result{code: 1, stdout: m6Doc(t, found)}
	}}).Execute
	if err := m6ConfigCases()[0].run(s.m6Scope(m6ConfigCategory, "repository-config-ignored"), t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 2 {
		t.Fatalf("lint ran in %q", dirs)
	}
	for i, name := range []string{".eslintrc.json", "pyproject.toml", ".shellcheckrc", ".hadolint.yaml"} {
		_, plain := os.Stat(filepath.Join(dirs[0], name))
		_, configured := os.Stat(filepath.Join(dirs[1], name))
		if plain == nil || configured != nil {
			t.Errorf("configuration %d %s: plain %v, configured %v", i, name, plain, configured)
		}
	}
}

// TestM6ConfigRowStopsWhenARepositoryIsNotAdopted: a twin the candidate
// refuses to adopt, or a lint that cannot run, is a failed row.
func TestM6ConfigRowStopsWhenARepositoryIsNotAdopted(t *testing.T) {
	t.Parallel()
	for _, reply := range []m5Reply{
		{prefix: []string{commandInit}, answer: m5Answer(result{code: 2})},
		{prefix: []string{m6LintCommand}, answer: m5Answer(result{err: ErrAcceptance})},
	} {
		s := m6TestSuite(t)
		s.execute = m5Script(t, s.suite, reply, m6InitReply()).Execute
		if err := m6ConfigCases()[0].run(s.m6Scope(m6ConfigCategory, "refused"), t.Context()); err == nil {
			t.Errorf("%q: the row passed", reply.prefix)
		}
	}
	s := m6TestSuite(t)
	s.root = t.TempDir()
	s.execute = m5Script(t, s.suite, m6InitReply()).Execute
	if err := m6ConfigCases()[0].run(s.m6Scope(m6ConfigCategory, "no-configs"), t.Context()); err == nil {
		t.Fatal("the row passed without its configuration fixtures")
	}
}
