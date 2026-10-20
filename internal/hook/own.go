package hook

import (
	"context"
	"slices"
)

// commitAnswer is what git answered about the commits the command created:
// the paths its commits changed, the merges it made and every commit it
// made, ok false when git could not tell them.
type commitAnswer struct {
	asked  bool
	ok     bool
	paths  map[string]bool
	merges []Commit
	mine   []Commit
}

// commitMemory is what a post event keeps of what git answered, so git is
// asked once whichever of the move and the merge needs it first: the
// commits of the command, the move of HEAD the event read, which names the
// files of an amend at HEAD, and the rewrite of the HEAD the pre event
// started from.
type commitMemory struct {
	commitAnswer

	moved   moves
	rewrite rewrite
}

// ownCommits answers created through the memory of the post event, asking
// git only the first time; without one it asks every time.
func (run execution) ownCommits(ctx context.Context, dependencies Dependencies, before Baseline) commitAnswer {
	if run.own == nil {
		return run.created(ctx, dependencies, before)
	}
	if !run.own.asked {
		run.own.commitAnswer = run.created(ctx, dependencies, before)
	}
	return run.own.commitAnswer
}

// notOwn keeps, of the paths whose bytes git wrote for a merge in progress,
// what it merged or the side of HEAD, those no commit of the command
// changed: when the command moved HEAD before it started the merge, a path
// its own commit changed holds that change, so it stays judged, as does a
// path its amend changed against the HEAD it rewrote, a file the amend
// restored to the bytes of its parent included (untouched), and when git
// cannot tell which commits the command created, each such path is
// answered apart as untold.
func (run execution) notOwn(ctx context.Context, dependencies Dependencies, before Baseline, changes Changes, merged []string) ([]string, []string) {
	if len(merged) == 0 || changes.Head == before.Head {
		return merged, nil
	}
	own := run.ownCommits(ctx, dependencies, before)
	if !own.ok {
		return nil, merged
	}
	merged = slices.DeleteFunc(merged, func(path string) bool { return own.paths[path] })
	if len(merged) == 0 {
		return merged, nil
	}
	rewriter, known := run.rewriterOnce(ctx, dependencies, before, own.mine)
	return run.untouched(ctx, dependencies, before, rewriter, known, merged)
}
