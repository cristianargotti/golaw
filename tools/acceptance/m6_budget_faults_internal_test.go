package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/toolchain"
)

// TestM6BudgetRowsStopWhenTheTwinIsNotAdopted: no row times a repository
// the candidate refused to adopt.
func TestM6BudgetRowsStopWhenTheTwinIsNotAdopted(t *testing.T) {
	t.Parallel()
	for _, c := range m6BudgetCases() {
		s := m6TestSuite(t)
		s.shared.now = m6Clock(time.Millisecond)
		s.execute = m5Script(t, s.suite, m5Reply{prefix: []string{commandInit}, answer: m5Answer(result{code: 2})}).Execute
		if err := c.run(s.m6Scope(m6BudgetCategory, c.name), t.Context()); err == nil {
			t.Errorf("%s: a row ran over a twin the candidate refused to adopt", c.name)
		}
	}
}

// TestM6BudgetRowsRefuseARunThatDoesNotAnswerCleanly: a timing of a hook
// that fails is no timing, so the sampled rows fail on it.
func TestM6BudgetRowsRefuseARunThatDoesNotAnswerCleanly(t *testing.T) {
	t.Parallel()
	for _, got := range []result{{code: 2, stderr: "golaw: denied\n"}, {err: ErrAcceptance}} {
		for _, c := range m6BudgetCases()[:2] {
			s := m6TestSuite(t)
			s.shared.now = m6Clock(time.Millisecond)
			s.execute = m5Script(t, s.suite, m6InitReply(), m5Reply{prefix: []string{commandHook, agentClaude}, answer: m5Answer(got)}).Execute
			if err := c.run(s.m6Scope(m6BudgetCategory, c.name), t.Context()); err == nil {
				t.Errorf("%s: the row passed over the answer %+v", c.name, got)
			}
		}
	}
}

// TestM6SlowToolStopsWhenTheSleeperIsNotBuilt: a build that fails, or that
// leaves no executable, records no receipt and times nothing.
func TestM6SlowToolStopsWhenTheSleeperIsNotBuilt(t *testing.T) {
	t.Parallel()
	for name, got := range map[string]result{"build fails": {code: 1, stderr: "no Go\n"}, "build leaves nothing": {}} {
		s := m6TestSuite(t)
		s.shared.now = m6Clock(1600 * time.Millisecond)
		build := m5Reply{program: s.goTool, prefix: []string{goBuild}, answer: m5Answer(got)}
		s.execute = m5Script(t, s.suite, build, m6InitReply()).Execute
		if err := m6BudgetCases()[2].run(s.m6Scope(m6BudgetCategory, "slow-tool-unbuilt"), t.Context()); err == nil {
			t.Errorf("%s: the row passed", name)
		}
	}
}

func TestM6JudgeKilledRefusesAFailedAnswer(t *testing.T) {
	t.Parallel()
	for name, got := range map[string]result{
		"red the candidate did not run": {err: ErrAcceptance},
		"red exit 2":                    {code: 2, stderr: "golaw: denied\n"},
		"red not JSON":                  {stdout: "{"},
	} {
		if err := m6JudgeKilled(1600*time.Millisecond, got); err == nil {
			t.Errorf("%s: the answer passed", name)
		}
	}
}

func TestM6ReceiptRefusesAToolThatIsNotThere(t *testing.T) {
	t.Parallel()
	tool, found := toolchain.Lookup(m6SleeperTool)
	if !found {
		t.Fatal("the tool table pins no ruff")
	}
	dir := t.TempDir()
	if err := m6Receipt(dir, tool, filepath.Join(dir, "absent")); err == nil {
		t.Fatal("red a receipt recorded a file that does not exist")
	}
}
