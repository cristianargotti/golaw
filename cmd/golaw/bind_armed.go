package main

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

// armedBySetup reports whether golaw setup armed the hooks directory git
// runs the hooks of dir from: a setup receipt of the state directory names
// it. Setup is what arms a rung (decision 10, A11), so the git hooks
// another tool installed, which read the lefthook.yml init writes, run no
// golaw rung before a person ran golaw setup. The hooks directory keys the
// answer, not the worktree root, because every worktree of a repository
// runs the hooks of its common directory.
func (b binding) armedBySetup(ctx context.Context, dir string) bool {
	dirs, err := b.dirs()
	if err != nil {
		return false
	}
	out, err := runner.Run(ctx, runner.Command{Name: "git", Args: []string{"rev-parse", "--path-format=absolute", "--git-path", "hooks"}, Dir: dir, Env: b.environ()})
	if err != nil || out.Code != 0 {
		return false
	}
	hooks := realPath(strings.TrimSpace(string(out.Stdout)))
	receipts, err := filepath.Glob(filepath.Join(filepath.Dir(setup.ReceiptPath(dirs.State, "")), "*.json"))
	if err != nil {
		return false
	}
	for _, path := range receipts {
		if r, err := setup.ReadReceipt(path); err == nil && realPath(r.HooksPath) == hooks {
			return true
		}
	}
	return false
}

// realPath is a path with its links resolved, or the clean path when it
// cannot be resolved.
func realPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil { // fallback: a hooks directory not made yet compares by its clean form
		return resolved
	}
	return filepath.Clean(path)
}
