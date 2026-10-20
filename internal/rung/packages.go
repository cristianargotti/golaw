package rung

import (
	"context"
	"fmt"
	pathpkg "path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// The suffix of a Go file and the files of the module that make every
// package affected.
const (
	goSuffix = ".go"
	goMod    = "go.mod"
	goSum    = "go.sum"
)

// goCommit reports whether the commit can touch a Go package: the index
// holds a module at the root and a changed path is a Go input, so a commit
// of documents asks go nothing.
func (p *precommit) goCommit() bool {
	if p.scan.Index.Class(goMod) == model.ClassUnknown {
		return false
	}
	dirs := p.goDirs()
	return slices.ContainsFunc(changedPaths(p.scan.Input.Changes), func(path string) bool { return touchesGo(path, dirs) })
}

// affected answers the packages the commit touches, asked of go in the tree
// the steps read, so the package graph is the one of what the commit
// records and never of what the worktree holds unstaged.
func (p *precommit) affected(ctx context.Context, dir string) (runner.AffectedSet, error) {
	set, err := runner.Affected(ctx, dir, changedPaths(p.scan.Input.Changes), runner.UserEnv(p.deps.Environ))
	if err != nil {
		return runner.AffectedSet{}, fmt.Errorf("name the affected packages: %w", err)
	}
	return set, nil
}

// goDirs are the directories that hold a Go file of the index, each a
// package.
func (p *precommit) goDirs() map[string]bool {
	dirs := map[string]bool{}
	for _, f := range p.scan.Index.Select(func(path string) bool { return strings.HasSuffix(path, goSuffix) }) {
		dirs[pathpkg.Dir(f.Path)] = true
	}
	return dirs
}

// touchesGo reports whether a path is a Go input: a Go file, go.mod or
// go.sum, or a file of a package directory, a testdata directory belonging
// to the package above it.
func touchesGo(path string, dirs map[string]bool) bool {
	return strings.HasSuffix(path, goSuffix) || path == goMod || path == goSum || dirs[packageDir(path)]
}

// ignoredGo lists the Go files git ignores, those of an ignored directory
// included, because go reads every one of them in place while the commit
// and the push carry none. A name that starts with a dot or an underscore
// is left out: go never reads such a file.
func ignoredGo(ctx context.Context, deps Dependencies) ([]string, error) {
	out, err := plumbing(ctx, deps, "ls-files", "-z", "--others", "--ignored", "--exclude-standard", "--", "*"+goSuffix)
	if err != nil {
		return nil, err
	}
	if out.Code != 0 {
		return nil, fmt.Errorf("%w: git ls-files: %s", source.ErrGit, strings.TrimSpace(string(out.Stderr)))
	}
	var files []string
	for path := range strings.SplitSeq(string(out.Stdout), "\x00") {
		if base := pathpkg.Base(path); path != "" && !strings.HasPrefix(base, ".") && !strings.HasPrefix(base, "_") {
			files = append(files, path)
		}
	}
	return files, nil
}

// packageDir is the directory of the package a path belongs to: its own
// directory, or the one above the first testdata directory it lies under.
func packageDir(path string) string {
	parts := strings.Split(path, "/")
	if i := slices.Index(parts, "testdata"); i >= 0 {
		return pathpkg.Join(append([]string{"."}, parts[:i]...)...)
	}
	return pathpkg.Dir(path)
}

// changedPaths are the paths the changes touch, the old path of a rename or
// a deletion included, each once and in order.
func changedPaths(changes []model.Change) []string {
	var out []string
	for _, c := range changes {
		for _, path := range []string{c.OldPath, c.NewPath} {
			if path != "" && !slices.Contains(out, path) {
				out = append(out, path)
			}
		}
	}
	return out
}
