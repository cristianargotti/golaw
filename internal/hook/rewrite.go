package hook

import (
	"context"
	"slices"
)

// amends reports whether a command text runs git commit --amend plainly
// (gitInvocations), the one way the hook reads a commit made on the parents
// of the HEAD the pre event started from as a rewrite of that HEAD: a reset
// followed by a commit makes the same commit while what the reset brought
// is no write of the command, and an amend a script runs is not read, a
// stated limit.
func amends(command string) bool {
	for _, words := range gitInvocations(command) {
		if words[0] == "commit" && slices.ContainsFunc(words[1:], func(word string) bool { return bare(word) == "--amend" }) {
			return true
		}
	}
	return false
}

// unrewritten keeps, of the paths a move brought from commits the command
// did not create, those no rewrite of the command changed. A commit the
// command created on the parents of the HEAD the pre event started from,
// an amend, holds every difference from that HEAD by the command's choice,
// so each path the rewrite changed is judged as the command's write, a file
// the amended commit drops from its delta, restored to the bytes of its
// parent, included. The paths of the rewrite are those of the move when the
// rewriting commit is HEAD, and asked of git diff-tree once otherwise; the
// paths git could not tell are answered apart.
func (run execution) unrewritten(ctx context.Context, dependencies Dependencies, before Baseline, mine []Commit, arrived []string) ([]string, []string) {
	rewriter, known := run.rewriter(ctx, dependencies, before, mine, arrived)
	switch {
	case !known:
		return nil, arrived
	case rewriter == "":
		return arrived, nil
	case rewriter == run.repository.Head:
		return nil, nil
	case dependencies.Moved == nil:
		return nil, arrived
	}
	rewritten, err := dependencies.Moved(ctx, run.repository, before.Head, rewriter)
	if err != nil {
		return nil, arrived
	}
	return slices.DeleteFunc(slices.Clone(arrived), func(path string) bool {
		_, changed := rewritten[path]
		return changed
	}), nil
}

// rewriter is the commit of the command that rewrites the HEAD the pre
// event started from, a commit made on its parents by a command text that
// amends, empty when none does.
// The parents of that HEAD are asked of git once, only when a path is left
// to name and no commit of the command builds on that HEAD; it is false when
// git cannot name them.
func (run execution) rewriter(ctx context.Context, dependencies Dependencies, before Baseline, mine []Commit, arrived []string) (string, bool) {
	if !before.Amend || len(arrived) == 0 || len(mine) == 0 || before.Head == "" || dependencies.Parents == nil {
		return "", true
	}
	for _, commit := range mine {
		if slices.Contains(commit.Parents, before.Head) {
			return "", true
		}
	}
	parents, err := dependencies.Parents(ctx, run.repository, before.Head)
	if err != nil {
		return "", false
	}
	for _, commit := range mine {
		if slices.Equal(commit.Parents, parents) {
			return commit.Name, true
		}
	}
	return "", true
}
