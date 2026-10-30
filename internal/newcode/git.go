package newcode

import (
	"context"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

const (
	gitProgram = "git"
	headRef    = "HEAD"
	remoteHead = "refs/remotes/origin/HEAD"
	forEachRef = "for-each-ref"
)

// remoteDefaults are the branches a CI checkout fetches without setting
// origin/HEAD, in the order a remote names its default one.
func remoteDefaults() []string {
	return []string{"refs/remotes/origin/main", "refs/remotes/origin/master"}
}

type environKey struct{}

// WithEnviron carries the environment the package's git runs with: the
// package may not read the process environment, and the hook variables of
// a surrounding git must never point its git at another repository
// (spec 5.7), so they leave here through runner.UserEnv.
func WithEnviron(ctx context.Context, environ []string) context.Context {
	return context.WithValue(ctx, environKey{}, runner.UserEnv(environ))
}

// git runs one plumbing command in root; a context without an environment
// inherits the process one, as runner.Run does for a nil Env.
func git(ctx context.Context, root string, args ...string) (runner.Output, error) {
	var environ []string
	if carried, ok := ctx.Value(environKey{}).([]string); ok {
		environ = carried
	}
	out, err := runner.Run(ctx, runner.Command{
		Name: gitProgram,
		Args: append([]string{"-c", "core.quotePath=false"}, args...),
		Dir:  root,
		Env:  environ,
	})
	if err != nil {
		return out, fmt.Errorf("%w: git %s: %w", ErrDiff, args[0], err)
	}
	return out, nil
}

// failure reads every unexpected exit the same way, with the first line
// git wrote to its error stream.
func failure(command string, out runner.Output) error {
	first, _, _ := strings.Cut(strings.TrimSpace(string(out.Stderr)), "\n")
	return fmt.Errorf("%w: git %s exited %d: %s", ErrDiff, command, out.Code, first)
}

func trimmed(out runner.Output) string {
	return strings.TrimSpace(string(out.Stdout))
}

// resolve names the commit of ref; --end-of-options keeps an option-shaped
// ref a revision, so it can only fail to resolve.
func resolve(ctx context.Context, root, ref string) (string, error) {
	const command = "rev-parse"
	out, err := git(ctx, root, command, "--verify", "--quiet", "--end-of-options", ref+"^{commit}")
	switch {
	case err != nil:
		return "", err
	case out.Code == 0:
		return trimmed(out), nil
	case out.Code == 1 && len(out.Stderr) == 0:
		return "", fmt.Errorf("%w: %s names no commit of this repository", ErrBase, ref)
	}
	return "", failure(command, out)
}

// MergeBase is the fork of ref and HEAD, where the new code of a pull
// request starts (M6.8 item 4).
func MergeBase(ctx context.Context, root, ref string) (string, error) {
	const command = "merge-base"
	commit, err := resolve(ctx, root, ref)
	if err != nil {
		return "", err
	}
	out, err := git(ctx, root, command, "--", commit, headRef)
	switch {
	case err != nil:
		return "", err
	case out.Code == 0:
		return trimmed(out), nil
	case out.Code == 1:
		return "", fmt.Errorf("%w: %s and HEAD share no commit here: fetch the base with its history, a shallow clone has none", ErrBase, ref)
	}
	return "", failure(command, out)
}

// LastTag is the nearest tag reachable from HEAD, the base of --since-tag;
// git answers 128 both for no tag and for an untagged history.
func LastTag(ctx context.Context, root string) (string, error) {
	out, err := git(ctx, root, "describe", "--tags", "--abbrev=0", headRef)
	if err != nil {
		return "", err
	}
	if out.Code != 0 {
		return "", fmt.Errorf("%w: no tag is reachable from HEAD, name --base <ref> instead", ErrBase)
	}
	return trimmed(out), nil
}

// DefaultBranch is the remote default branch, empty when the remote names
// none. for-each-ref answers 0 with nothing for an absent ref, where
// symbolic-ref would die; a CI checkout fetches the remote branches without
// origin/HEAD, so origin/main and then origin/master stand in for it.
func DefaultBranch(ctx context.Context, root string) (string, error) {
	target, err := refQuery(ctx, root, "--format=%(symref)", remoteHead)
	if err != nil || target != "" {
		return target, err
	}
	for _, ref := range remoteDefaults() {
		names, err := refQuery(ctx, root, "--format=%(refname)", ref)
		if err != nil {
			return "", err
		}
		// A pattern also matches the refs below it, hence the equal line.
		for name := range strings.SplitSeq(names, "\n") {
			if name == ref {
				return ref, nil
			}
		}
	}
	return "", nil
}

func refQuery(ctx context.Context, root, format, ref string) (string, error) {
	out, err := git(ctx, root, forEachRef, format, ref)
	if err != nil {
		return "", err
	}
	if out.Code != 0 {
		return "", failure(forEachRef, out)
	}
	return trimmed(out), nil
}
