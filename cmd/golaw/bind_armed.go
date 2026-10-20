package main

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

// armedBySetup reports whether golaw setup armed the hooks git runs for
// dir: a setup receipt of the state directory names the hooks directory git
// answers there, and its root belongs to the repository of dir. Setup is
// what arms a rung (decision 10, A11), so the git hooks another tool
// installed, which read the lefthook.yml init writes, run no golaw rung
// before a person ran golaw setup. The hooks directory and the common
// directory key the answer, not the worktree root, because every worktree
// of a repository runs the hooks of its common directory, and a hooks
// directory several repositories share through core.hooksPath is armed
// only for the one setup ran in.
func (b binding) armedBySetup(ctx context.Context, dir string) bool {
	dirs, err := b.dirs()
	if err != nil {
		return false
	}
	paths := gitPaths(ctx, dir, b.environ(), "--git-path", "hooks", "--git-common-dir")
	if len(paths) != 2 {
		return false
	}
	receipts, err := filepath.Glob(filepath.Join(filepath.Dir(setup.ReceiptPath(dirs.State, "")), "*.json"))
	if err != nil {
		return false
	}
	for _, path := range receipts {
		if r, err := setup.ReadReceipt(path); err == nil && realPath(r.HooksPath) == paths[0] && b.sameRepository(ctx, r.Root, paths[1]) {
			return true
		}
	}
	return false
}

// sameRepository reports whether the worktree at root, the one a receipt
// names, has the common directory common. Its git runs without the hook
// variables, which would point it at the repository of the hook whatever
// its working directory; a root that is gone answers false.
func (b binding) sameRepository(ctx context.Context, root, common string) bool {
	theirs := gitPaths(ctx, root, runner.UserEnv(b.environ()), "--git-common-dir")
	return len(theirs) == 1 && theirs[0] == common
}

// gitPaths asks git in dir for the paths args name, absolute and with their
// links resolved, in the order rev-parse prints them, one a line; none when
// git cannot answer.
func gitPaths(ctx context.Context, dir string, env []string, args ...string) []string {
	out, err := runner.Run(ctx, runner.Command{Name: "git", Args: append([]string{"rev-parse", "--path-format=absolute"}, args...), Dir: dir, Env: env})
	if err != nil || out.Code != 0 {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(string(out.Stdout), "\n"), "\n")
	for i, line := range lines {
		lines[i] = realPath(strings.TrimSuffix(line, "\r"))
	}
	return lines
}

// realPath is a path with its links resolved, or the clean path when it
// cannot be resolved.
func realPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil { // fallback: a hooks directory not made yet compares by its clean form
		return resolved
	}
	return filepath.Clean(path)
}
