package rung

import (
	"context"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/runner"
)

// vetStep names the go vet step, and noPackage is the line of a Go step
// over a commit that touches no package.
const (
	vetStep   = "vet"
	noPackage = "skipped, the commit touches no Go package"
)

// goSteps runs the steps that read Go packages: it chooses the tree that
// holds what the index holds for every Go input, names there the packages
// the commit touches, runs the steps there and removes a copy it made.
func (p *precommit) goSteps(ctx context.Context) Outcome {
	t, out, ok := p.tree(ctx)
	if !ok {
		return out
	}
	for _, step := range []func(context.Context, goTree) Outcome{p.golangci, p.vet, p.test} {
		out = join(out, step(ctx, t))
	}
	if err := t.close(); err != nil {
		out = join(out, stepFailed(treeStep, err))
	}
	return out
}

// goCommand is a go command over the affected packages, in the tree, with
// the user environment and the tree named by PWD (treeEnv).
func (p *precommit) goCommand(t goTree, args ...string) runner.Command {
	return runner.Command{Name: "go", Args: slices.Concat(args, t.set.Patterns), Dir: t.dir, Env: p.treeEnv(t)}
}

// treeEnv is the environment of a tool that reads the tree: the user
// environment with PWD naming the tree by its real path, the last value of
// a name winning. A tool takes PWD for its working directory when it names
// the same directory, so a worktree a person reached through a linked
// directory would read as a path through a link, where golangci-lint 2.14.0
// drops every issue of --new-from-patch.
func (p *precommit) treeEnv(t goTree) []string {
	return append(runner.UserEnv(p.deps.Environ), "PWD="+t.dir)
}

// vet runs go vet over the affected packages and refuses what it reports
// on a line the commit adds, a build error there included; the rest a
// parent may already hold, so it is named and left to the pipeline.
func (p *precommit) vet(ctx context.Context, t goTree) Outcome {
	if len(t.set.Patterns) == 0 {
		return passed(vetStep, noPackage)
	}
	out, err := runner.Run(ctx, p.goCommand(t, "vet"))
	switch {
	case err != nil:
		return stepFailed(vetStep, err)
	case out.Code == 0:
		return passed(vetStep, fmt.Sprintf("go vet found nothing in %d affected packages", len(t.set.Packages)))
	}
	added, err := p.introduced(ctx)
	if err != nil {
		return stepFailed(vetStep, err)
	}
	kept, inherited := attributed(out, added)
	result := Outcome{}
	if len(kept) > 0 {
		result = Outcome{Code: codeFindings, Lines: slices.Concat([]string{stepLine(vetStep, "go vet reports what the affected packages break; fix it before the commit")}, kept)}
	}
	if len(inherited) > 0 || len(kept) == 0 {
		result = join(result, Outcome{Lines: slices.Concat([]string{stepLine(vetStep, "go vet reports problems on lines the commit does not add, which the pipeline judges")}, inherited)})
	}
	return result
}
