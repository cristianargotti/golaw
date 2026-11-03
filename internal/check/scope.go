package check

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

// Scope is the change golaw check judges.
type Scope struct {
	Base  string   // the ref named or resolved, empty for the whole tree
	Fork  string   // the merge base of Base and HEAD
	Whole bool     // no base resolved: every file of the tree counts as changed
	Paths []string // the changed slash paths, deletions included, sorted
}

// Changes answers the slash paths, sorted and once each, the worktree
// changes since fork: committed, staged, unstaged and untracked. A deletion
// stays, so the packages that imported a removed file stay affected.
// Porcelain diff is used because it compares content, where diff-index
// would also name a file whose stat alone changed.
func Changes(ctx context.Context, root, fork string, env []string) ([]string, error) {
	diffed, err := gitPaths(ctx, root, env, "diff", "--name-only", "-z", "--no-renames", "--no-relative", "--no-ext-diff", fork, "--")
	if err != nil {
		return nil, err
	}
	untracked, err := gitPaths(ctx, root, env, "ls-files", "--others", "--exclude-standard", "--full-name", "-z")
	if err != nil {
		return nil, err
	}
	paths := slices.Concat(diffed, untracked)
	slices.Sort(paths)
	return slices.Compact(paths), nil
}

func gitPaths(ctx context.Context, root string, env []string, args ...string) ([]string, error) {
	out, err := runner.Run(ctx, runner.Command{
		Name: "git", Args: append([]string{"-c", "core.quotePath=false"}, args...), Dir: root, Env: env,
	})
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", args[0], err)
	}
	if out.Code != 0 {
		first, _, _ := strings.Cut(strings.TrimSpace(string(out.Stderr)), "\n")
		return nil, fmt.Errorf("%w: git %s exited %d: %s", ErrGit, args[0], out.Code, first)
	}
	var paths []string
	for name := range strings.SplitSeq(string(out.Stdout), "\x00") {
		if name != "" {
			paths = append(paths, name)
		}
	}
	return paths, nil
}

// Select answers the changed paths that match.
func (s Scope) Select(match func(name string) bool) []string {
	var out []string
	for _, name := range s.Paths {
		if match(name) {
			out = append(out, name)
		}
	}
	return out
}

// Changed reports whether the path is part of the change; the whole tree
// holds every path.
func (s Scope) Changed(name string) bool {
	if s.Whole {
		return true
	}
	_, found := slices.BinarySearch(s.Paths, name)
	return found
}

// GoFile reports whether a slash path is a Go source file.
func GoFile(name string) bool { return path.Ext(name) == ".go" }

// Migration reports whether a slash path is a SQL migration of the
// directory make migrate-lint reads.
func Migration(name string) bool {
	return path.Dir(name) == "db/migrations" && path.Ext(name) == ".sql"
}
