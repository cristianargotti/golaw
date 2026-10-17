package hook

import (
	"context"
	"slices"
)

// fromCommits keeps, of the paths a move of HEAD brought, those a commit
// the command created changed, and names every other one in a notice that
// asks for no fix, because the move brought it from a commit someone else
// or an earlier moment made, which the pipeline judges; when git cannot
// tell which commits the command created, each path it brought is a gap.
func (run execution) fromCommits(ctx context.Context, dependencies Dependencies, before Baseline, changed, brought []string) ([]string, []Gap) {
	if len(brought) == 0 {
		return changed, nil
	}
	created, ok := run.created(ctx, dependencies, before)
	if !ok {
		return slices.DeleteFunc(changed, among(brought)), pathGaps(brought, causeCommits)
	}
	arrived := slices.DeleteFunc(slices.Clone(brought), func(path string) bool { return created[path] })
	return slices.DeleteFunc(changed, among(arrived)), pathGaps(arrived, causeArrived)
}

// created are the paths the commits the command created changed: the
// commits since the HEAD of the pre event that are no merge, committed in
// the whole second the pre event started or later, as git records whole
// seconds, by the identity git commits as now. It is false when git cannot
// answer the commits or the identity.
func (run execution) created(ctx context.Context, dependencies Dependencies, before Baseline) (map[string]bool, bool) {
	if dependencies.Commits == nil || dependencies.Committer == nil {
		return nil, false
	}
	committer, err := dependencies.Committer(ctx, run.repository)
	if err != nil {
		return nil, false
	}
	from := before.At.Unix()
	commits, err := dependencies.Commits(ctx, run.repository, before.Head, run.repository.Head, from)
	if err != nil {
		return nil, false
	}
	paths := map[string]bool{}
	for _, commit := range commits {
		if commit.Time < from || commit.Committer != committer {
			continue
		}
		for _, path := range commit.Paths {
			paths[path] = true
		}
	}
	return paths, true
}
