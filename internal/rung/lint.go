package rung

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// The two golangci-lint steps, the tool they run, the name and mode of the
// file that holds the staged patch beside the lock of the rung, and the line
// of a format step with no Go file staged.
const (
	fmtStep   = "fmt"
	lintStep  = "lint"
	linter    = "golangci-lint"
	patchName = "pre-commit.patch"
	patchMode = 0o600
	noGoFile  = "skipped, no Go file staged"
)

// golangci runs the two golangci-lint steps over the commit, the pinned
// version verified once, whenever the commit touches a package, a change
// of go.mod alone included: the format of the staged Go files and the
// linters over the affected packages.
func (p *precommit) golangci(ctx context.Context, t goTree) Outcome {
	if len(t.set.Patterns) == 0 {
		return join(passed(fmtStep, noGoFile), passed(lintStep, noPackage))
	}
	env := p.treeEnv(t)
	path, err := verified(ctx, linter, env)
	if err != nil {
		return stepFailed(fmtStep+" and "+lintStep, err)
	}
	return join(p.format(ctx, t, path, env, stagedGo(p.scan.Input.Targets)), p.lint(ctx, t, path, env))
}

// format runs golangci-lint fmt --diff over the staged Go files. It answers
// whole files, so a diff is named and never refused: a line the commit did
// not touch is no finding of the commit (section 7), and the lint step
// reports the formatters on the staged lines through --new-from-patch.
func (p *precommit) format(ctx context.Context, t goTree, path string, env, files []string) Outcome {
	if len(files) == 0 {
		return passed(fmtStep, noGoFile)
	}
	out, err := runner.Run(ctx, runner.Command{Name: path, Args: slices.Concat([]string{"fmt", "--diff"}, files), Dir: t.dir, Env: env})
	switch {
	case err != nil:
		return stepFailed(fmtStep, err)
	case out.Code == 0:
		return passed(fmtStep, fmt.Sprintf("golangci-lint fmt finds the %d staged Go files formatted", len(files)))
	case out.Code == 1:
		return passed(fmtStep, "golangci-lint fmt would change the staged Go files as whole files; the lint step judges the formatters on the staged lines, the pipeline the rest")
	default:
		return broke(fmtStep, linter, out)
	}
}

// lint runs golangci-lint over the affected packages and keeps the issues
// of the lines the staged patch adds. While a merge is in progress that
// patch holds what the merge brought, which the pipeline judges.
func (p *precommit) lint(ctx context.Context, t goTree, path string, env []string) Outcome {
	if p.scan.Index.MergeHead() != "" {
		return passed(lintStep, "skipped, a merge is in progress and its staged patch holds what the merge brought, which the pipeline judges")
	}
	patch, err := p.patch(ctx)
	if err != nil {
		return stepFailed(lintStep, err)
	}
	args := slices.Concat([]string{"run", "--new-from-patch=" + patch}, t.set.Patterns)
	out, err := runner.Run(ctx, runner.Command{Name: path, Args: args, Dir: t.dir, Env: env})
	result := linted(len(t.set.Packages), out, err)
	if gone := os.Remove(patch); gone != nil && !errors.Is(gone, fs.ErrNotExist) {
		result = join(result, passed(lintStep, "could not remove the staged patch: "+gone.Error()))
	}
	return result
}

// linted reads what golangci-lint answered: 0 nothing on the staged lines,
// 1 issues there.
func linted(packages int, out runner.Output, err error) Outcome {
	switch {
	case err != nil:
		return stepFailed(lintStep, err)
	case out.Code == 0:
		return passed(lintStep, fmt.Sprintf("golangci-lint found no issue on the staged lines of %d affected packages", packages))
	case out.Code == 1:
		return refusedBy(lintStep, "golangci-lint reports issues on the staged lines; fix them before the commit", out)
	default:
		return broke(lintStep, linter, out)
	}
}

// patch writes the staged diff of the index git commits through beside the
// lock of the rung, where only the holder of the lock writes and the next
// run replaces it, and answers its path. The prefixes are named because
// golangci-lint places the staged lines of a file through the a/ and b/ of
// the patch, which diff.noprefix drops and diff.mnemonicPrefix renames.
func (p *precommit) patch(ctx context.Context) (string, error) {
	out, err := plumbing(ctx, p.deps, "diff", "--cached", "--no-color", "--no-ext-diff", "-M", "--src-prefix=a/", "--dst-prefix=b/")
	if err != nil {
		return "", err
	}
	if out.Code != 0 {
		return "", fmt.Errorf("%w: git diff: %s", source.ErrGit, strings.TrimSpace(string(out.Stderr)))
	}
	path := filepath.Join(p.repo.GitDir, "golaw", patchName)
	if err := os.WriteFile(path, out.Stdout, patchMode); err != nil {
		return "", fmt.Errorf("write the staged patch: %w", err)
	}
	return path, nil
}

// stagedGo lists the staged Go files in the form of the operating system,
// relative to the root of the tree the tools read.
func stagedGo(targets []model.File) []string {
	var out []string
	for _, f := range targets {
		if strings.HasSuffix(f.Path, goSuffix) {
			out = append(out, filepath.FromSlash(f.Path))
		}
	}
	return out
}
