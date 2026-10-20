package hook

import (
	"context"
	"slices"
)

// commitMemory is what git answered about the commits the command created:
// the paths its commits changed and the merges it made, ok false when git
// could not tell them. A post event keeps one, so git is asked once
// whichever of the move and the merge needs it first.
type commitMemory struct {
	asked  bool
	ok     bool
	paths  map[string]bool
	merges []Commit
}

// ownCommits answers created through the memory of the post event, asking
// git only the first time; without one it asks every time.
func (run execution) ownCommits(ctx context.Context, dependencies Dependencies, before Baseline) commitMemory {
	if run.own == nil {
		return run.created(ctx, dependencies, before)
	}
	if !run.own.asked {
		*run.own = run.created(ctx, dependencies, before)
	}
	return *run.own
}

// notOwn keeps, of the paths whose bytes git wrote for a merge in progress,
// what it merged or the side of HEAD, those no commit of the command
// changed: when the command moved HEAD before it started the merge, a path
// its own commit changed holds that change, so it stays judged, and when
// git cannot tell which commits the command created, each such path is
// answered apart as untold.
func (run execution) notOwn(ctx context.Context, dependencies Dependencies, before Baseline, changes Changes, merged []string) ([]string, []string) {
	if len(merged) == 0 || changes.Head == before.Head {
		return merged, nil
	}
	own := run.ownCommits(ctx, dependencies, before)
	if !own.ok {
		return nil, merged
	}
	return slices.DeleteFunc(merged, func(path string) bool { return own.paths[path] }), nil
}
