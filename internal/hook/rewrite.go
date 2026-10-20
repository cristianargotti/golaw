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
	if len(arrived) == 0 {
		return arrived, nil
	}
	rewriter, known := run.rewriterOnce(ctx, dependencies, before, mine)
	if known && rewriter != "" && rewriter == run.repository.Head {
		return nil, nil
	}
	return run.untouched(ctx, dependencies, before, rewriter, known, arrived)
}

// untouched keeps, of the paths, those the rewriting commit did not change
// against the HEAD the pre event started from, every one when no commit of
// the command rewrites it; when git could not tell the rewrite or its
// files, each path is answered apart.
func (run execution) untouched(ctx context.Context, dependencies Dependencies, before Baseline, rewriter string, known bool, paths []string) ([]string, []string) {
	switch {
	case !known:
		return nil, paths
	case rewriter == "":
		return paths, nil
	}
	files, told := run.rewrittenFiles(ctx, dependencies, before, rewriter)
	if !told {
		return nil, paths
	}
	return slices.DeleteFunc(slices.Clone(paths), func(path string) bool {
		_, changed := files[path]
		return changed
	}), nil
}

// rewrite is what git answered about the rewrite of the HEAD the pre event
// started from: the commit of the command that rewrites it, empty when none
// does, known false when git could not name the parents of that HEAD, and,
// once git named them, the files that commit changed against that HEAD.
type rewrite struct {
	asked  bool
	known  bool
	commit string
	told   bool
	files  map[string]string
}

// rewriterOnce answers rewriter through the memory of the post event,
// asking git only the first time, because the move and the merge may both
// need it; without a memory it asks every time.
func (run execution) rewriterOnce(ctx context.Context, dependencies Dependencies, before Baseline, mine []Commit) (string, bool) {
	if run.own == nil {
		return run.rewriter(ctx, dependencies, before, mine)
	}
	if !run.own.rewrite.asked {
		commit, known := run.rewriter(ctx, dependencies, before, mine)
		run.own.rewrite = rewrite{asked: true, known: known, commit: commit}
	}
	return run.own.rewrite.commit, run.own.rewrite.known
}

// rewrittenFiles are the files the rewriting commit changed against the
// HEAD the pre event started from: those of the move the event remembers
// when that commit is HEAD, and asked of git diff-tree once otherwise; false
// when git cannot tell them or no binding asks it.
func (run execution) rewrittenFiles(ctx context.Context, dependencies Dependencies, before Baseline, rewriter string) (map[string]string, bool) {
	switch {
	case run.own != nil && rewriter == run.repository.Head && run.own.moved.known:
		return run.own.moved.paths, true
	case run.own != nil && run.own.rewrite.told:
		return run.own.rewrite.files, true
	case dependencies.Moved == nil:
		return nil, false
	}
	files, err := dependencies.Moved(ctx, run.repository, before.Head, rewriter)
	if err != nil {
		return nil, false
	}
	if run.own != nil {
		run.own.rewrite.told, run.own.rewrite.files = true, files
	}
	return files, true
}

// rewriter is the commit of the command that rewrites the HEAD the pre
// event started from, a commit made on its parents by a command text that
// amends, empty when none does.
// The parents of that HEAD are asked of git once, only when a path is left
// to name and no commit of the command builds on that HEAD; it is false when
// git cannot name them.
func (run execution) rewriter(ctx context.Context, dependencies Dependencies, before Baseline, mine []Commit) (string, bool) {
	if !before.Amend || len(mine) == 0 || before.Head == "" || dependencies.Parents == nil {
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
