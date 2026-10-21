package rung

import (
	"context"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

// failPrefix starts the line go test prints for a package that failed.
const failPrefix = "FAIL\t"

// heldAtHead answers whether every test that fails in the packages the
// commit touches already fails at HEAD, and names each package with its
// tests: such a failure is no fault of the commit, so the rung leaves it
// to the pipeline (the owner's rule that a rung refuses only what it
// recognizes with certainty). Only failures go test names a test for are
// compared, once more at HEAD in a copy of its tree; a package that does
// not build, a failure no test names, an unborn branch or a HEAD that
// cannot be copied or tested leaves the refusal as it stands.
func (p *precommit) heldAtHead(ctx context.Context, out runner.Output, touched []string) ([]string, bool) {
	tests := failedTests(out)
	var names []string
	for _, pkg := range touched {
		if len(tests[pkg]) == 0 {
			return nil, false
		}
		names = append(names, tests[pkg]...)
	}
	if p.repo.Head == "" {
		return nil, false
	}
	base, ok := p.testAtHead(ctx, names, touched)
	if !ok {
		return nil, false
	}
	held := failedTests(base)
	named := make([]string, 0, len(touched))
	for _, pkg := range touched {
		if slices.ContainsFunc(tests[pkg], func(name string) bool { return !slices.Contains(held[pkg], name) }) {
			return nil, false
		}
		named = append(named, pkg+" ("+strings.Join(tests[pkg], ", ")+")")
	}
	return named, true
}

// testAtHead runs the named tests of the packages in a copy of the tree of
// HEAD, outside the repository, and removes the copy; a copy that cannot
// be made, or a go test that cannot start, answers no run.
func (p *precommit) testAtHead(ctx context.Context, names, packages []string) (runner.Output, bool) {
	copied, err := runner.Materialize(ctx, runner.MaterializeInput{
		Dir: p.deps.Dir, Env: p.deps.Environ, Rev: p.repo.Head, Parent: treesDir(p.deps), Bounds: runner.DefaultBounds(),
	})
	if err != nil {
		return runner.Output{}, false
	}
	args := slices.Concat([]string{"test", "-race", "-run", "^(" + strings.Join(names, "|") + ")$"}, packages)
	out, err := runner.Run(ctx, runner.Command{Name: "go", Args: args, Dir: copied.Dir, Env: p.treeEnv(goTree{dir: copied.Dir})})
	if closeErr := copied.Close(); closeErr != nil {
		return runner.Output{}, false
	}
	return out, err == nil
}

// failedTests are the top-level tests go test names in a --- FAIL line,
// by the import path of the package whose FAIL line follows them: go test
// prints the output of each package whole, its result line last.
func failedTests(out runner.Output) map[string][]string {
	byPackage := map[string][]string{}
	var pending []string
	for line := range strings.SplitSeq(string(out.Stdout), "\n") {
		if name, ok := strings.CutPrefix(strings.TrimLeft(line, " \t"), "--- FAIL: "); ok {
			name, _, _ = strings.Cut(name, " ")
			name, _, _ = strings.Cut(name, "/")
			if !slices.Contains(pending, name) {
				pending = append(pending, name)
			}
			continue
		}
		if rest, ok := strings.CutPrefix(line, failPrefix); ok {
			pkg, _, _ := strings.Cut(rest, "\t")
			pkg, _, _ = strings.Cut(pkg, " ")
			byPackage[pkg] = append(byPackage[pkg], pending...)
		}
		if strings.HasPrefix(line, failPrefix) || strings.HasPrefix(line, "ok  \t") {
			pending = nil
		}
	}
	return byPackage
}
