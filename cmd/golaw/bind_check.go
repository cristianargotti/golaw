package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/check"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/newcode"
	"github.com/cristianargotti/golaw/internal/runner"
)

// The gates of golaw check, in the order the report names them: the
// cheap ones first, as make ci orders its targets.
const (
	gateFormat     = "format"
	gateVet        = "vet"
	gateLint       = "lint"
	gateTest       = "test"
	gateCoverage   = "coverage"
	gateScan       = "scan"
	gateGenerate   = "generate"
	gateMigrations = "migrations"
)

// The tool make migrate-lint runs, the capability that arms it, and the
// pattern of every package of the module.
const (
	squawkTool   = "squawk"
	dbCapability = "db"
	allPackages  = "./..."
)

// checkTools are the programs the gates run; an empty one is missing and
// skips the gates that need it.
type checkTools struct{ golang, lint, squawk string }

// checkPlan is one golaw check run: the policy in force, the change, the
// affected packages and the tools, computed once and read by every gate.
type checkPlan struct {
	a        *app
	root     string
	policy   *model.EffectivePolicy
	scope    check.Scope
	note     string
	module   string // the module path of the root go.mod, empty without one
	affected runner.AffectedSet
	env      []string
	tools    checkTools
	temp     string // the directory of the coverage profile, removed by close
}

// checkPlan loads the policy, resolves the base and the change, and finds
// the affected packages and the tools.
func (a *app) checkPlan(ctx context.Context, dir string, opts checkOptions) (*checkPlan, error) {
	b := a.bind()
	loaded, err := b.loaded(ctx, dir, a.now())
	if err != nil {
		return nil, err
	}
	if checkoutErr := enrolledCheckout(loaded); checkoutErr != nil {
		return nil, checkoutErr
	}
	environ := b.environ()
	p := &checkPlan{a: a, root: loaded.Root, policy: loaded.Policy, env: runner.UserEnv(environ)}
	ctx = newcode.WithEnviron(ctx, environ)
	if scopeErr := p.resolveScope(ctx, opts); scopeErr != nil {
		return nil, scopeErr
	}
	p.module = moduleAt(p.root)
	p.affected = p.affectedSet(ctx)
	p.tools = checkTools{golang: onPath(environ, "go"), lint: onPath(environ, linterTool), squawk: onPath(environ, squawkTool)}
	if p.temp, err = os.MkdirTemp("", "golaw-check-"); err != nil {
		return nil, fmt.Errorf("make the profile directory: %w", err)
	}
	return p, nil
}

// resolveScope finds the base, as golaw coverage --new-code does, and the
// change since its fork; without a base the whole tree is judged.
func (p *checkPlan) resolveScope(ctx context.Context, opts checkOptions) error {
	base := opts.base
	if base == "" {
		branch, err := newcode.DefaultBranch(ctx, p.root)
		if err != nil {
			return fmt.Errorf("the remote default branch: %w", err)
		}
		if branch == "" {
			p.whole("no remote default branch names a base here: the whole tree is judged, name --base <ref> to judge a change")
			return nil
		}
		base = branch
	}
	fork, err := newcode.MergeBase(ctx, p.root, base)
	if opts.baseNamed && err != nil {
		return fmt.Errorf("%w: %w", model.ErrUsage, err)
	}
	if err != nil {
		p.whole("the base " + base + " has no merge base with HEAD here: the whole tree is judged")
		return nil //nolint:nilerr // an unresolvable default base judges the whole tree, as make lint does
	}
	paths, err := check.Changes(ctx, p.root, fork, p.env)
	if err != nil {
		return fmt.Errorf("the change since %s: %w", base, err)
	}
	p.scope = check.Scope{Base: base, Fork: fork, Paths: paths}
	return nil
}

func (p *checkPlan) whole(note string) {
	p.scope = check.Scope{Whole: true}
	p.note = note
}

// affectedSet answers the packages the change reaches; a listing go cannot
// make, such as one over a broken go.mod, reaches the whole module, so the
// compiler names the defect instead of a gate passing unseen.
func (p *checkPlan) affectedSet(ctx context.Context) runner.AffectedSet {
	all := runner.AffectedSet{All: true, Patterns: []string{allPackages}}
	if p.module == "" || p.scope.Whole {
		return all
	}
	set, err := runner.Affected(ctx, p.root, p.scope.Paths, p.env)
	if err != nil {
		return all
	}
	return set
}

// moduleAt answers the module path of the go.mod at the root, empty when
// the root holds none or it names no module.
func moduleAt(root string) string {
	data, err := os.ReadFile(filepath.Join(root, initModuleFile)) //nolint:gosec // the go.mod of the repository root
	if err != nil {
		return ""
	}
	module, err := newcode.Module(data)
	if err != nil {
		return ""
	}
	return module
}

// gates are the gates of the run, in report order.
func (p *checkPlan) gates() []check.Gate {
	return []check.Gate{p.formatGate, p.vetGate, p.lintGate, p.testGate, p.scanGate, p.generateGate, p.migrationsGate}
}

// report names the base and the results.
func (p *checkPlan) report(results []check.Result) check.Report {
	return check.Report{Base: p.scope.Base, Whole: p.scope.Whole, Note: p.note, Results: results}
}

// close removes the profile directory; a directory left behind is only a
// temporary file, so its error is dropped on purpose.
func (p *checkPlan) close() {
	_ = os.RemoveAll(p.temp) //nolint:errcheck // a temporary directory the system cleans anyway
}

// goReason answers why the Go gates do not apply, empty when they do.
func (p *checkPlan) goReason() string {
	switch {
	case !p.policy.Has(goCapability):
		return "the go capability is off"
	case p.module == "":
		return "the repository root holds no go.mod"
	case len(p.affected.Patterns) == 0:
		return "no Go package changed"
	}
	return ""
}

// existing keeps the changed paths still present in the worktree; a path
// whose stat fails otherwise stays, so its tool names the defect.
func (p *checkPlan) existing(paths []string) []string {
	var out []string
	for _, name := range paths {
		_, err := os.Stat(filepath.Join(p.root, filepath.FromSlash(name)))
		if !errors.Is(err, os.ErrNotExist) {
			out = append(out, name)
		}
	}
	return out
}
