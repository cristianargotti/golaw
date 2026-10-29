package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The m6-coverage rows of M6.10 row (4): the command, its flags, the two
// reports and the numbers Sonar's line model records over each of them.
const (
	m6CoverageCategory = "m6-coverage"
	m6CoverageCommand  = "coverage"
	m6NewCodeFlag      = "--new-code"
	m6InputFlag        = "--input"
	m6BaseFlag         = "--base"
	m6PlainInputFlag   = "--plain-input"
	m6ParityReport     = "profile.cov"
	m6StrictReport     = "strict.cov"
	m6ParityNumber     = "63.6%"
	m6StrictNumber     = "81.8%"
	m6StrictLabel      = "strict floors"
	m6ParityLabel      = "pipeline parity"
)

// m6Coverage is a fixture checkout and the commit starting its new code.
type m6Coverage struct{ repo, base string }

// m6Packages are the measured packages, named rather than ./... because
// init --apply renders a test-only package whose imports need the network.
func m6Packages() []string {
	return []string{"./a", "./b", "./integration", "./gen"}
}

// m6CoverageCases are the m6-coverage rows in report order.
func m6CoverageCases() []m6Case {
	return []m6Case{
		{name: "parity-number", run: (*m6Suite).m6ParityNumberCase},
		{name: "strict-differs", run: (*m6Suite).m6StrictDiffersCase},
		{name: "missing-report", run: (*m6Suite).m6MissingReportCase},
		{name: "stale-report", run: (*m6Suite).m6StaleReportCase},
	}
}

// m6CoverageRepository adopts the fixture base, records its commit as the
// start of the new code period and commits the measured packages over it.
func (s *m6Suite) m6CoverageRepository(ctx context.Context, name string) (m6Coverage, error) {
	base, err := m6Fixture(s.root, "coverage/base")
	if err != nil {
		return m6Coverage{}, err
	}
	repo, err := s.m6Repository(ctx, name, base, nil)
	if err != nil {
		return m6Coverage{}, err
	}
	got := s.execute(ctx, command{program: s.git, args: []string{m2RevParse, m2Head}, dir: repo, env: s.env})
	if headErr := success(got); headErr != nil {
		return m6Coverage{}, fmt.Errorf("read the base commit of %s: %w", name, headErr)
	}
	added, err := m6Fixture(s.root, "coverage/new")
	if err != nil {
		return m6Coverage{}, err
	}
	if err := m6WriteFiles(repo, added); err != nil {
		return m6Coverage{}, err
	}
	if err := s.m6Commit(ctx, repo, "feat: add the measured packages"); err != nil {
		return m6Coverage{}, err
	}
	return m6Coverage{repo: repo, base: strings.TrimSpace(got.stdout)}, nil
}

// m6Profile writes report in the pipeline's shape (survey 4.1) or strict.
func (s *m6Suite) m6Profile(ctx context.Context, repo, report string, strict bool) error {
	args := []string{"test"}
	if strict {
		args = append(args, "-coverpkg="+strings.Join(m6Packages(), ","))
	}
	args = append(append(args, "-coverprofile", report), m6Packages()...)
	if err := success(s.execute(ctx, command{program: s.goTool, args: args, dir: repo, env: s.env})); err != nil {
		return fmt.Errorf("write %s: %w", report, err)
	}
	return nil
}

// m6NewCode measures the new code of cov from report.
func (s *m6Suite) m6NewCode(ctx context.Context, cov m6Coverage, report string) result {
	return s.m5Golaw(ctx, cov.repo, m6CoverageCommand, m6NewCodeFlag, m6InputFlag, report, m6BaseFlag, cov.base)
}

// m6Measured is the fixture checkout of name with reports written.
func (s *m6Suite) m6Measured(ctx context.Context, name string, reports ...string) (m6Coverage, error) {
	cov, err := s.m6CoverageRepository(ctx, name)
	for _, report := range reports {
		if err != nil {
			break
		}
		err = s.m6Profile(ctx, cov.repo, report, report == m6StrictReport)
	}
	return cov, err
}

// m6ParityNumberCase proves the pipeline's profile is under Sonar way.
func (s *m6Suite) m6ParityNumberCase(ctx context.Context) error {
	cov, err := s.m6Measured(ctx, "coverage-parity", m6ParityReport)
	if err != nil {
		return err
	}
	return m6JudgeNumber(s.m6NewCode(ctx, cov, m6ParityReport), 1, m6ParityNumber)
}

// m6StrictDiffersCase proves -coverpkg credits the line of b in a.
func (s *m6Suite) m6StrictDiffersCase(ctx context.Context) error {
	cov, err := s.m6Measured(ctx, "coverage-strict", m6ParityReport, m6StrictReport)
	if err != nil {
		return err
	}
	if err := m6JudgeNumber(s.m6NewCode(ctx, cov, m6StrictReport), 0, m6StrictNumber); err != nil {
		return err
	}
	return m6JudgeLabels(s.m5Golaw(ctx, cov.repo, m6CoverageCommand, m6InputFlag, m6StrictReport, m6PlainInputFlag, m6ParityReport))
}

// m6MissingReportCase proves a report never written is Unmeasured.
func (s *m6Suite) m6MissingReportCase(ctx context.Context) error {
	cov, err := s.m6Measured(ctx, "coverage-missing")
	if err != nil {
		return err
	}
	return m6JudgeUnmeasuredReport(s.m6NewCode(ctx, cov, "absent.cov"))
}

// m6StaleReportCase proves a report older than an in-scope Go file is
// Unmeasured (M6.8 item 6): an edit never passes on the old numbers.
func (s *m6Suite) m6StaleReportCase(ctx context.Context) error {
	cov, err := s.m6Measured(ctx, "coverage-stale", m6ParityReport)
	if err != nil {
		return err
	}
	later := s.shared.now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(cov.repo, "a", "new.go"), later, later); err != nil {
		return fmt.Errorf("touch the new code after its report: %w", err)
	}
	return m6JudgeUnmeasuredReport(s.m6NewCode(ctx, cov, m6ParityReport))
}

// m6JudgeNumber wants the exit and the recorded percent of a measure.
func m6JudgeNumber(got result, code int, number string) error {
	switch {
	case got.err != nil:
		return fmt.Errorf("run coverage --new-code: %w", got.err)
	case got.code != code:
		return fmt.Errorf("coverage --new-code exited %d, want %d: %q %q: %w", got.code, code, got.stdout, got.stderr, ErrAcceptance)
	case !strings.Contains(got.stdout, number):
		return fmt.Errorf("coverage --new-code printed %q, want the recorded %s: %w", got.stdout, number, ErrAcceptance)
	}
	return nil
}

// m6JudgeLabels wants both labeled profiles printed with other numbers.
func m6JudgeLabels(got result) error {
	strict, parity := strings.Index(got.stdout, m6StrictLabel), strings.Index(got.stdout, m6ParityLabel)
	switch {
	case got.err != nil:
		return fmt.Errorf("run coverage over both profiles: %w", got.err)
	case got.code == 2:
		return fmt.Errorf("coverage over both profiles exited 2: %q: %w", got.stderr, ErrAcceptance)
	case strict < 0 || parity < 0:
		return fmt.Errorf("coverage printed %q, want the labels %q and %q: %w", got.stdout, m6StrictLabel, m6ParityLabel, ErrAcceptance)
	case m6Section(got.stdout, strict, parity) == m6Section(got.stdout, parity, strict):
		return fmt.Errorf("both profiles print the same numbers: -coverpkg credited no line: %w", ErrAcceptance)
	}
	return nil
}

// m6Section is the text under the label at from, without the label's line,
// which names its own flag and alone would make any two sections differ.
func m6Section(out string, from, other int) string {
	end := len(out)
	if other > from {
		end = other
	}
	_, body, _ := strings.Cut(out[from:end], "\n")
	return strings.TrimSpace(body)
}

// m6JudgeUnmeasuredReport wants exit 2 and the cause of a bad report.
func m6JudgeUnmeasuredReport(got result) error {
	switch {
	case got.err != nil:
		return fmt.Errorf("run coverage --new-code: %w", got.err)
	case got.code != 2 || strings.TrimSpace(got.stderr) == "":
		return fmt.Errorf("a missing or stale report answered %d with stderr %q, want exit 2 and its cause: %w", got.code, got.stderr, ErrAcceptance)
	}
	return nil
}
