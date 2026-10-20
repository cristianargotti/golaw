package rung

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

// variable answers the value the environment gives a name, the last one
// when it repeats, names compared without regard to case as Windows
// compares them, and whether it is set.
func variable(environ []string, name string) (string, bool) {
	value, set := "", false
	for _, entry := range environ {
		if key, v, ok := strings.Cut(entry, "="); ok && strings.EqualFold(key, name) {
			value, set = v, true
		}
	}
	return value, set
}

// indexFile is the index git commits through, absolute: the GIT_INDEX_FILE
// git gives the hook, which may be relative to the worktree root, made
// absolute against it; empty when git gives none and the repository's own
// index is the one.
func indexFile(deps Dependencies) string {
	file, set := variable(deps.Environ, "GIT_INDEX_FILE")
	if !set || file == "" || filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(deps.Dir, file)
}

// plumbing asks git a question of the kit's own in the worktree root, with
// the environment git gave the hook, so the answer is about the repository
// and the index of the commit or the push; any exit status is an answer.
func plumbing(ctx context.Context, deps Dependencies, args ...string) (runner.Output, error) {
	out, err := runner.Run(ctx, runner.Command{
		Name: "git", Args: slices.Concat([]string{"-c", "core.quotePath=false"}, args), Dir: deps.Dir, Env: deps.Environ,
	})
	if err != nil {
		return out, fmt.Errorf("git %s: %w", args[0], err)
	}
	return out, nil
}
