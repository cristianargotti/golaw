package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// m6CoverageFault runs every m6-coverage case over replies that break one
// step and wants each case to fail, never to pass over the broken step.
func m6CoverageFault(t *testing.T, name string, prepare func(*m6Suite), replies func(*m6Suite) []m5Reply) {
	t.Helper()
	for _, c := range m6CoverageCases() {
		s := m6TestSuite(t)
		if prepare != nil {
			prepare(s)
		}
		s.execute = m5Script(t, s.suite, replies(s)...).Execute
		if err := c.run(s.m6Scope(m6CoverageCategory, c.name), t.Context()); err == nil {
			t.Errorf("%s passed over %s", c.name, name)
		}
	}
}

// m6CoverageOverRoot points the suite at a root holding only the fixture
// directories named, so a missing one is a read failure.
func m6CoverageOverRoot(t *testing.T, dirs ...string) func(*m6Suite) {
	t.Helper()
	return func(s *m6Suite) {
		root := t.TempDir()
		for _, dir := range dirs {
			files, err := m6Fixture(s.root, dir)
			if err != nil {
				t.Fatal(err)
			}
			for name, body := range files {
				checkedWrite(t, filepath.Join(root, filepath.FromSlash(m6FixtureRoot), dir, filepath.FromSlash(name)+".esc"), string(body))
			}
		}
		s.root = root
	}
}

func TestM6CoverageRowsFailWhenAStepFails(t *testing.T) {
	t.Parallel()
	recorded := func(s *m6Suite) []m5Reply {
		var tests [][]string
		return m6CoverageReplies(t, s, &tests)
	}
	m6CoverageFault(t, "a missing fixture base", m6CoverageOverRoot(t), recorded)
	m6CoverageFault(t, "a missing set of new packages", m6CoverageOverRoot(t, "coverage/base"), recorded)
	m6CoverageFault(t, "a refused init", nil, func(*m6Suite) []m5Reply {
		return []m5Reply{{prefix: []string{commandInit}, answer: m5Answer(result{code: 2})}}
	})
	m6CoverageFault(t, "an unreadable base commit", nil, func(s *m6Suite) []m5Reply {
		return append([]m5Reply{{program: s.git, prefix: []string{m2RevParse}, answer: m5Answer(result{code: 128})}}, recorded(s)...)
	})
	m6CoverageFault(t, "a refused commit of the new packages", nil, func(s *m6Suite) []m5Reply {
		refused := m5Reply{program: s.git, prefix: commitArgs("feat: add the measured packages"), answer: m5Answer(result{code: 1})}
		return append([]m5Reply{refused}, recorded(s)...)
	})
	m6CoverageFault(t, "a new package written over a directory", nil, func(*m6Suite) []m5Reply {
		return []m5Reply{{prefix: []string{commandInit}, answer: m5After(func(t *testing.T, c command) {
			t.Helper()
			m6FakeInit(t, c)
			if err := os.MkdirAll(filepath.Join(c.dir, "b", "b.go", "held"), dirMode); err != nil {
				t.Fatal(err)
			}
			checkedWrite(t, filepath.Join(c.dir, "b", "b.go", "held", "keep"), "x\n")
		}, result{})}}
	})
}

func TestM6CoverageMeasuredRowsFailOnTheirOwnStep(t *testing.T) {
	t.Parallel()
	failing := func(s *m6Suite) []m5Reply {
		var tests [][]string
		return append([]m5Reply{{program: s.goTool, prefix: []string{"test"}, answer: m5Answer(result{code: 1})}}, m6CoverageReplies(t, s, &tests)...)
	}
	for _, c := range m6CoverageCases() {
		if c.name == "missing-report" {
			continue
		}
		s := m6TestSuite(t)
		s.execute = m5Script(t, s.suite, failing(s)...).Execute
		if err := c.run(s.m6Scope(m6CoverageCategory, c.name), t.Context()); err == nil {
			t.Errorf("%s passed over a failed go test", c.name)
		}
	}
	s := m6TestSuite(t)
	var tests [][]string
	wrong := m5Reply{prefix: []string{m6CoverageCommand, m6NewCodeFlag, m6InputFlag, m6StrictReport}, answer: m5Answer(result{code: 1, stdout: "new code: 63.6% (7/11)\n"})}
	s.execute = m5Script(t, s.suite, append([]m5Reply{m6InitReply(), wrong}, m6CoverageReplies(t, s, &tests)[1:]...)...).Execute
	if err := s.m6Scope(m6CoverageCategory, "strict").m6StrictDiffersCase(t.Context()); err == nil ||
		!strings.Contains(err.Error(), "coverage --new-code exited 1, want 0") {
		t.Errorf("strict-differs over the parity number on the strict report = %v", err)
	}
	s = m6TestSuite(t)
	gone := m5Reply{program: s.goTool, prefix: []string{"test"}, answer: m5After(func(t *testing.T, c command) {
		t.Helper()
		checkedWrite(t, filepath.Join(c.dir, m6ParityReport), "mode: set\n")
		if err := os.Remove(filepath.Join(c.dir, "a", "new.go")); err != nil {
			t.Fatal(err)
		}
	}, result{})}
	s.execute = m5Script(t, s.suite, append([]m5Reply{m6InitReply(), gone}, m6CoverageReplies(t, s, &tests)[2:]...)...).Execute
	if err := s.m6Scope(m6CoverageCategory, "stale").m6StaleReportCase(t.Context()); err == nil ||
		!strings.Contains(err.Error(), "touch the new code after its report") {
		t.Errorf("stale-report over a new file it could not touch = %v", err)
	}
	if m6JudgeLabels(result{err: ErrAcceptance}) == nil || m6JudgeUnmeasuredReport(result{err: ErrAcceptance}) == nil {
		t.Error("a process that never ran was judged")
	}
}
