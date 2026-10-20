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
	for _, step := range []func(context.Context, goTree) Outcome{p.vet} {
		out = join(out, step(ctx, t))
	}
	if err := t.close(); err != nil {
		out = join(out, stepFailed(treeStep, err))
	}
	return out
}

// goCommand is a go command over the affected packages, in the tree, with
// the user environment.
func (p *precommit) goCommand(t goTree, args ...string) runner.Command {
	return runner.Command{Name: "go", Args: slices.Concat(args, t.set.Patterns), Dir: t.dir, Env: runner.UserEnv(p.deps.Environ)}
}

// vet runs go vet over the affected packages; a package that does not
// build is refused as go vet refuses it.
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
	default:
		return refusedBy(vetStep, "go vet reports what the affected packages break; fix it before the commit", out)
	}
}
