package main

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// TestM6MissingRowsStopWhenTheTwinIsNotAdopted: every row answers the
// refused adoption instead of judging a repository that does not exist.
func TestM6MissingRowsStopWhenTheTwinIsNotAdopted(t *testing.T) {
	t.Parallel()
	for _, c := range m6MissingCases() {
		s := m6TestSuite(t)
		s.execute = m5Script(t, s.suite, m5Reply{prefix: []string{commandInit}, answer: m5Answer(result{code: 2})}).Execute
		if err := c.run(s.m6Scope(m6MissingCategory, c.name), t.Context()); err == nil {
			t.Errorf("%s: a row ran over a twin the candidate refused to adopt", c.name)
		}
	}
}

// TestM6MissingRowsReportTheCandidateFailure: a doctor that fails or
// prints no JSON, a setup that refuses to arm and a post event that blocks
// each fail their row.
func TestM6MissingRowsReportTheCandidateFailure(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name  string
		index int
		reply m5Reply
	}{
		{"doctor fails", 0, m5Reply{prefix: []string{m5DoctorCommand}, answer: m5Answer(result{code: 1})}},
		{"doctor prints no JSON", 0, m5Reply{prefix: []string{m5DoctorCommand}, answer: m5Answer(result{stdout: "{"})}},
		{"post event blocks", 1, m5Reply{prefix: []string{commandHook, agentClaude}, answer: m5Answer(result{stdout: `{"decision":"block"}` + "\n"})}},
		{"setup refuses", 2, m5Reply{prefix: []string{"setup"}, answer: m5Answer(result{code: 1})}},
		{"lint passes", 3, m5Reply{prefix: []string{m6LintCommand}, answer: m5Answer(result{stdout: "{}\n"})}},
	}
	for _, row := range rows {
		s := m6TestSuite(t)
		s.execute = m5Script(t, s.suite, append([]m5Reply{row.reply}, m6MissingReplies(t)...)...).Execute
		c := m6MissingCases()[row.index]
		if err := c.run(s.m6Scope(m6MissingCategory, c.name), t.Context()); err == nil {
			t.Errorf("%s: the row passed", row.name)
		}
	}
}

// TestM6RungNoticeStopsWhenStagingFails: a file git refuses to stage is
// never handed to the rung.
func TestM6RungNoticeStopsWhenStagingFails(t *testing.T) {
	t.Parallel()
	s := m6TestSuite(t)
	f := m6HookFiles()[0]
	refused := m5Reply{program: s.git, prefix: []string{gitAdd, f.path}, answer: m5Answer(result{code: 128})}
	s.execute = m5Script(t, s.suite, append([]m5Reply{refused}, m6MissingReplies(t)...)...).Execute
	if err := s.m6Scope(m6MissingCategory, "rung-unstaged").m6RungNotice(t.Context()); err == nil {
		t.Fatal("the rung ran over a file git refused to stage")
	}
}

// TestM6MissingHelpersNameTheirCause: an unknown tool, a source the twin
// lacks, a directory outside git and an environment without a home are
// errors, never an empty expectation.
func TestM6MissingHelpersNameTheirCause(t *testing.T) {
	t.Parallel()
	if _, _, err := m6Notice("absent-tool", "a.py"); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("an unknown tool = %v", err)
	}
	if _, err := m6HookBody(m6HookFile{path: "a.py", source: "absent.py", tool: "ruff"}); !errors.Is(err, ErrAcceptance) {
		t.Fatalf("an absent source = %v", err)
	}
	if _, err := m6Needed(t.Context(), t.TempDir(), nil); err == nil {
		t.Fatal("a directory outside git was inventoried")
	}
	s := m6TestSuite(t)
	s.execute = m5Script(t, s.suite, m6InitReply()).Execute
	repo, err := s.m6Twin(t.Context(), "homeless", surfacefix.Corrected, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m6Needed(t.Context(), repo, nil); err == nil {
		t.Fatal("a tools directory resolved without a home")
	}
}
