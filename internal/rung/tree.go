package rung

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// treeStep names the choice of the tree the Go steps read.
const treeStep = "tree"

// goTree is where the steps that read Go packages run: the packages the
// commit touches, the directory that holds for them the bytes the index
// holds, and the removal of that directory when it is a copy.
type goTree struct {
	set   runner.AffectedSet
	dir   string
	close func() error
}

// tree chooses where the Go steps read the commit and names there the
// packages it touches: the worktree in place when it holds the bytes of
// the index for every Go input, a copy of the index materialized outside
// the repository otherwise, so the package graph and the tools read what
// the commit records (section 7). A copy the runner refuses leaves the Go
// steps to the pipeline (copied), and so does a Go file git ignores: in
// place the steps would read it, and a copy would lack it, which refuses
// every commit of a repository that ignores the code it generates.
func (p *precommit) tree(ctx context.Context) (goTree, Outcome, bool) {
	t := goTree{dir: p.deps.Dir, close: func() error { return nil }}
	if !p.goCommit() {
		return t, Outcome{}, true
	}
	ignored, err := ignoredGo(ctx, p.deps)
	if err != nil {
		return t, stepFailed(treeStep, err), false
	}
	if len(ignored) > 0 {
		return t, passed(treeStep, "the Go steps are left to the pipeline: the worktree holds Go files git ignores, which the commit does not record and a copy of the index lacks: "+named(ignored)), false
	}
	dirty, err := p.dirty(ctx)
	if err != nil {
		return t, stepFailed(treeStep, err), false
	}
	noted, ok := Outcome{}, true
	if dirty {
		t, noted, ok = p.copied(ctx, t)
	}
	if !ok {
		return t, noted, false
	}
	// The tools read the tree by its real path: golangci-lint 2.14.0 drops
	// every issue of --new-from-patch in a directory reached through a link.
	if t.dir, err = filepath.EvalSymlinks(t.dir); err == nil {
		t.set, err = p.affected(ctx, t.dir)
	}
	if err != nil {
		return t, join(noted, stepFailed("packages", errors.Join(err, t.close()))), false
	}
	return t, noted, true
}

// copied materializes the index under the trees of the rungs, after a
// sweep of those an ended rung left (a sweep that fails is a line and no
// refusal). A copy the runner refuses as over its bounds or holding what
// it cannot copy, a link out of the tree or a link on a host that makes
// none, leaves the Go steps to the pipeline with the remedy, because a
// limit of the copy is no finding of the commit; a submodule is an empty
// directory in the copy.
func (p *precommit) copied(ctx context.Context, t goTree) (goTree, Outcome, bool) {
	noted := Outcome{}
	if swept := runner.Sweep(treesDir(p.deps)); swept != nil {
		noted = passed(treeStep, "could not sweep the trees an ended rung left: "+swept.Error())
	}
	copied, err := runner.Materialize(ctx, runner.MaterializeInput{Dir: p.deps.Dir, Env: p.deps.Environ, Parent: treesDir(p.deps), Bounds: runner.DefaultBounds()})
	switch {
	case errors.Is(err, runner.ErrBound) || errors.Is(err, model.ErrCannotJudge):
		limit := fmt.Sprintf("the Go steps are left to the pipeline: %v; stage the whole change, so the worktree holds what the index holds, and they read it in place", err)
		return t, join(noted, passed(treeStep, limit)), false
	case err != nil:
		return t, join(noted, stepFailed(treeStep, err)), false
	}
	t.dir, t.close = copied.Dir, copied.Close
	return t, noted, true
}

// dirty reports whether the worktree differs from the index git commits
// through in a Go input anywhere in the module: a file changed or deleted
// after it was staged, or a file git would add, that is a Go file, go.mod,
// go.sum or a file of a package directory of the index, an importer and a
// forward dependency alike, or a staged Go file whose worktree bytes are
// not the bytes of the index. Go reads all of them, so the worktree is read
// in place only when it holds what the commit records for every one.
func (p *precommit) dirty(ctx context.Context) (bool, error) {
	out, err := plumbing(ctx, p.deps, "--no-optional-locks", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames")
	if err != nil {
		return false, err
	}
	if out.Code != 0 {
		return false, fmt.Errorf("%w: git status: %s", source.ErrGit, strings.TrimSpace(string(out.Stderr)))
	}
	dirs := p.goDirs()
	for record := range strings.SplitSeq(string(out.Stdout), "\x00") {
		if len(record) >= 4 && record[1] != ' ' && touchesGo(record[3:], dirs) {
			return true, nil
		}
	}
	return p.converted(), nil
}

// converted reports whether a staged Go file holds other bytes in the
// worktree than in the index. git status compares through the line endings
// it converts on checkout (core.autocrlf, a text attribute) and calls such
// a file clean, while a formatter reading it in place would refuse every
// line of a file the commit records with LF.
func (p *precommit) converted() bool {
	for _, file := range p.scan.Input.Targets {
		if !strings.HasSuffix(file.Path, goSuffix) {
			continue
		}
		staged, err := p.scan.Index.Read(file.Path)
		if err != nil {
			return true
		}
		path := filepath.Join(p.deps.Dir, filepath.FromSlash(file.Path))
		info, err := os.Stat(path)
		if err != nil || info.Size() != int64(len(staged)) {
			return true
		}
		disk, err := os.ReadFile(path) //nolint:gosec // a staged path under the worktree root, the size of its staged blob
		if err != nil || !bytes.Equal(disk, staged) {
			return true
		}
	}
	return false
}

// treesDir is where the copies of the rungs live, outside every
// repository, one directory per copy, swept at the next start (A15).
func treesDir(deps Dependencies) string {
	return filepath.Join(deps.State, "runner", "trees")
}
