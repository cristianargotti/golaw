package main

import (
	"context"

	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// The category of row (1) of M6.10 and the command and flag of the lint
// over the non-Go surface.
const (
	m6SurfaceCategory = "m6-surface"
	m6LintCommand     = "lint"
	m6SurfaceFlag     = "--surface"
)

// m6SurfaceCases are the rows of the surface fixture: each twin under scan
// and under lint; only lint runs the surface tools, so only its rows need
// them.
func m6SurfaceCases() []m6Case {
	return []m6Case{
		m6SurfaceCase("red-scan", surfacefix.Red, false),
		m6SurfaceCase("red-lint", surfacefix.Red, true),
		m6SurfaceCase("corrected-scan", surfacefix.Corrected, false),
		m6SurfaceCase("corrected-lint", surfacefix.Corrected, true),
	}
}

// m6SurfaceCase is the row that judges twin under lint or scan.
func m6SurfaceCase(name string, twin surfacefix.Twin, lint bool) m6Case {
	return m6Case{name: name, tools: lint, run: func(s *m6Suite, ctx context.Context) error {
		return s.m6Judged(ctx, name, twin, lint)
	}}
}

// m6Report runs lint --surface or scan over repo with a JSON report. Scan
// reads no external tool, so it runs with the scope's own data home; lint
// runs with the shared tools.
func (s *m6Suite) m6Report(ctx context.Context, repo string, lint bool) result {
	if lint {
		return s.execute(ctx, command{program: s.binary, args: []string{m6LintCommand, m6SurfaceFlag, m2JSONFlag, "."}, dir: repo, env: s.m6ToolEnv()})
	}
	return s.execute(ctx, command{program: s.binary, args: []string{commandScan, m2JSONFlag, "."}, dir: repo, env: s.env})
}

// m6Judged adopts a repository of twin with the candidate's own init
// --apply, so the rule set in force is the strict one it writes, the set
// the expectations are written for, and judges its report.
func (s *m6Suite) m6Judged(ctx context.Context, name string, twin surfacefix.Twin, lint bool) error {
	repo, err := s.m6Twin(ctx, name, twin, nil)
	if err != nil {
		return err
	}
	return m6JudgeTwin(s.m6Report(ctx, repo, lint), twin, lint)
}
