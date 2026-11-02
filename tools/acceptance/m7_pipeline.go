package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
)

// The steps of the fresh repository a person and the generated pipeline
// take beyond golaw: the tools module directory init renders, the
// contract linter the generated CI installs with npm, and the contract a
// repository with no released version compares itself with.
const (
	m7GenCommand  = "gen"
	m7CheckFlag   = "--check"
	m7ToolsDir    = "tools"
	m7Redocly     = "redocly"
	m7OpenAPIBase = "api/openapi.yaml"
)

// m7Tidy writes the go.sum of the module in dir, the step a person runs
// after init renders the tools module and after golaw gen writes outputs
// that import the generators' runtimes: go tool, setup --gen and go test
// refuse a module without its requires and checksums.
func (s *m7Suite) m7Tidy(ctx context.Context, dir string) error {
	tidy := command{program: s.goTool, args: []string{"mod", "tidy"}, dir: dir, env: s.env, timeout: m2MeasurementTimeout}
	if err := success(s.execute(ctx, tidy)); err != nil {
		return fmt.Errorf("tidy the module of %s: %w", dir, err)
	}
	return nil
}

// m7MakeCI runs make ci of repo the way the generated pipeline does: the
// surface tools and redocly on PATH, the new code measured from the commit
// before the first golaw new, and the contract as its own base. A machine
// without those tools skips the row with the cause, as one without the
// pinned generators does.
func (s *m7Suite) m7MakeCI(ctx context.Context, repo m7Repo, flags ...string) (result, error) {
	if err := s.m6Tools(ctx); err != nil {
		return result{}, m7Skip(m6ToolsSkipped + err.Error())
	}
	redocly, err := s.lookup(m7Redocly)
	if err != nil {
		return result{}, m7Skip(fmt.Sprintf("%s unavailable: the generated CI installs @redocly/cli, put it on PATH: %v", m7Redocly, err))
	}
	args := slices.Concat(flags, []string{"ci", "COVERAGE_BASE=" + repo.base, "OPENAPI_BASE=" + m7OpenAPIBase})
	env := prependPath(s.m6ToolEnv(), filepath.Dir(redocly))
	return s.execute(ctx, command{program: s.m2.Make, args: args, dir: repo.dir, env: env, timeout: m2MeasurementTimeout}), nil
}

// m7Check runs golaw gen --check in dir.
func (s *m7Suite) m7Check(ctx context.Context, dir string) result {
	return s.m5Golaw(ctx, dir, m7GenCommand, m7CheckFlag)
}
