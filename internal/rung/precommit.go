package rung

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/stage"
)

// RunPreCommit is the pre-commit rung (section 7): the staged scan of what
// the commit introduces, the proposed policy judged apart, then, under the
// lock of the worktree, the declared toolchain at its pinned versions over
// the same commit. A refusal ends with one line in the language of the
// policy.
func RunPreCommit(ctx context.Context, deps Dependencies) Outcome {
	a, out, ok := gate(ctx, deps, PreCommit)
	if !ok {
		return out
	}
	p, err := newPrecommit(ctx, deps, a)
	if err != nil {
		return failed(PreCommit, a.loaded.Policy.Language, err)
	}
	return p.run(ctx)
}

// precommit is one run of the pre-commit rung: what its steps read.
type precommit struct {
	deps     Dependencies
	language string
	ladder   model.Ladder
	repo     source.Repo
	scan     stage.Result
	settled  settlement // what the scan leaves to the pipeline
	limits   []string   // the stated limits of the policy checks, left to the pipeline
}

// newPrecommit discovers the repository and scans the index git commits
// through, its decision settled so a file over the read bound is named
// instead of refusing; a refused proposal of golaw.yml is the error of the
// scan.
func newPrecommit(ctx context.Context, deps Dependencies, a armed) (*precommit, error) {
	repo, err := source.Discover(ctx, deps.Dir)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", deps.Dir, err)
	}
	scan, err := stage.Scan(ctx, stage.Input{
		Loaded: a.loaded, Repo: repo, Index: source.IndexOptions{IndexFile: indexFile(deps)},
		Registry: deps.Registry, Schemas: deps.Schemas,
	})
	if err != nil {
		return nil, fmt.Errorf("staged scan: %w", err)
	}
	limits := limited(scan.Run)
	s := settled(scan.Run)
	scan.Decision = s.decision
	return &precommit{deps: deps, language: a.loaded.Policy.Language, ladder: a.ladder, repo: repo, scan: scan, settled: s, limits: limits}, nil
}

// run reports the staged scan, takes the lock of the worktree so two
// commits never run the toolchain over one tree at once, runs the steps,
// and ends a refusal with the line of the human channel: the rule the scan
// names, or the pointer to the lines of the steps.
func (p *precommit) run(ctx context.Context) Outcome {
	lines, err := reportLines(p.scan.Run, p.scan.Decision)
	if err != nil {
		return failed(PreCommit, p.language, err)
	}
	lines = append(lines, p.settled.lines(PreCommit)...)
	lines = append(lines, limitLines(PreCommit, p.limits)...)
	lock, err := runner.Acquire(ctx, runner.LockPath(p.repo.GitDir, PreCommit), runner.RungWait)
	if err != nil {
		return failed(PreCommit, p.language, fmt.Errorf("%w: wait for the commit that holds it to end, then commit again", err))
	}
	tools := p.tools(ctx)
	if err := lock.Release(); err != nil {
		tools = join(tools, stepFailed("lock", err))
	}
	scanned := codeOf(p.scan.Decision)
	out := join(Outcome{Code: scanned, Lines: lines}, tools)
	switch {
	case scanned != codeClean:
		out.Lines = append(out.Lines, report.Human(p.language, p.scan.Decision))
	case out.Code != codeClean:
		out.Lines = append(out.Lines, refused(p.language, PreCommit))
	}
	return out
}

// tools runs the steps of the declared toolchain in order, then names the
// steps of section 7 this version does not ship.
func (p *precommit) tools(ctx context.Context) Outcome {
	out := Outcome{}
	for _, step := range []func(context.Context) Outcome{p.secrets, p.goSteps} {
		out = join(out, step(ctx))
	}
	return join(out, notShipped())
}
