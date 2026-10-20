package hook

import (
	"context"
	"slices"
)

// fromCommits keeps, of the paths a move of HEAD brought, those a commit
// the command created changed, and of a merge the command made, those it
// holds apart both from every parent and from the merge git makes of its
// parents by itself, its resolution of a conflict and its own files; it
// names every other one in a notice that asks for no fix, because the move
// brought it from a commit someone else or an earlier moment made, which
// the pipeline judges. When git cannot tell which commits the command
// created, each path it brought is a gap, and when it cannot make a merge
// of the command by itself, so is each path that merge holds apart from
// every parent.
func (run execution) fromCommits(ctx context.Context, dependencies Dependencies, before Baseline, changed, brought []string) ([]string, []Gap) {
	if len(brought) == 0 {
		return changed, nil
	}
	own := run.ownCommits(ctx, dependencies, before)
	if !own.ok {
		return slices.DeleteFunc(changed, among(brought)), pathGaps(brought, causeCommits)
	}
	created := own.paths
	unknown := among(run.resolved(ctx, dependencies, own.merges, brought, created, before.Options))
	var arrived, untold []string
	for _, path := range brought {
		switch {
		case created[path]:
		case unknown(path):
			untold = append(untold, path)
		default:
			arrived = append(arrived, path)
		}
	}
	changed = slices.DeleteFunc(changed, among(append(slices.Clone(arrived), untold...)))
	return changed, append(pathGaps(untold, causeRemergeUnknown), pathGaps(arrived, causeArrived)...)
}

// created are the paths the commits the command created changed, and apart
// the merges it made: the commits since the HEAD of the pre event,
// committed in the whole second the pre event started or later, as git
// records whole seconds, by the identity git commits as now. It is not ok
// when git cannot answer the commits or the identity.
func (run execution) created(ctx context.Context, dependencies Dependencies, before Baseline) commitMemory {
	if dependencies.Commits == nil || dependencies.Committer == nil {
		return commitMemory{asked: true}
	}
	committer, err := dependencies.Committer(ctx, run.repository)
	if err != nil {
		return commitMemory{asked: true}
	}
	from := before.At.Unix()
	commits, err := dependencies.Commits(ctx, run.repository, before.Head, run.repository.Head, from)
	if err != nil {
		return commitMemory{asked: true}
	}
	own := commitMemory{asked: true, ok: true, paths: map[string]bool{}}
	for _, commit := range commits {
		switch {
		case commit.Time < from || commit.Committer != committer:
		case commit.Merge:
			own.merges = append(own.merges, commit)
		default:
			for _, path := range commit.Paths {
				own.paths[path] = true
			}
		}
	}
	return own
}

// resolved adds to created the paths the move brought that a merge of the
// command holds apart both from every parent, as git log names them, and
// from the merge git makes of its parents by itself, asked of git once for
// every such merge and only when one holds such a path, and, when the
// command text gave strategy options, also apart from the merge git makes
// with them, asked once more for the paths still apart; it answers the
// paths it could not tell, every such path of a merge when git cannot make
// it by itself or no binding asks it.
func (run execution) resolved(ctx context.Context, dependencies Dependencies, merges []Commit, brought []string, created map[string]bool, options []string) []string {
	apart, names := mergesApart(merges, brought)
	if len(names) == 0 {
		return nil
	}
	remerged, known := run.remerged(ctx, dependencies, names)
	if !known {
		return flatten(apart, names)
	}
	for _, name := range names {
		apart[name] = slices.DeleteFunc(apart[name], func(path string) bool { return !slices.Contains(remerged[name], path) })
	}
	var unknown []string
	if len(options) > 0 {
		apart, unknown = run.remergedWith(ctx, dependencies, apart, names, options)
	}
	for _, path := range flatten(apart, names) {
		created[path] = true
	}
	return unknown
}

// mergesApart are, for each merge, the paths the move brought that it holds
// apart from every parent, and the names of the merges that hold one, in
// the order git log named them.
func mergesApart(merges []Commit, brought []string) (map[string][]string, []string) {
	movedHere := among(brought)
	apart := map[string][]string{}
	var names []string
	for _, merge := range merges {
		paths := slices.DeleteFunc(slices.Clone(merge.Paths), func(path string) bool { return !movedHere(path) })
		if len(paths) > 0 {
			apart[merge.Name] = paths
			names = append(names, merge.Name)
		}
	}
	return apart, names
}

// remergedWith keeps, of the paths each merge holds apart from the merge
// git makes of its parents by itself, those it also holds apart from that
// merge made with the strategy options, asked of git once for the merges
// that still hold one; when git cannot answer or no binding asks it, every
// such path is answered apart as unknown.
func (run execution) remergedWith(ctx context.Context, dependencies Dependencies, apart map[string][]string, names, options []string) (map[string][]string, []string) {
	var asked []string
	for _, name := range names {
		if len(apart[name]) > 0 {
			asked = append(asked, name)
		}
	}
	if len(asked) == 0 {
		return apart, nil
	}
	if dependencies.RemergedWith == nil {
		return nil, flatten(apart, asked)
	}
	again, err := dependencies.RemergedWith(ctx, run.repository, asked, options)
	if err != nil {
		return nil, flatten(apart, asked)
	}
	for _, name := range asked {
		apart[name] = slices.DeleteFunc(apart[name], func(path string) bool { return !slices.Contains(again[name], path) })
	}
	return apart, nil
}

// flatten lists the paths of the merges named, in their order.
func flatten(apart map[string][]string, names []string) []string {
	var paths []string
	for _, name := range names {
		paths = append(paths, apart[name]...)
	}
	return paths
}

// remerged asks git once, for the merges named, the paths each holds apart
// from the merge git makes of its parents by itself; it is false when git
// cannot answer or no binding asks it.
func (run execution) remerged(ctx context.Context, dependencies Dependencies, names []string) (map[string][]string, bool) {
	if dependencies.Remerged == nil {
		return nil, false
	}
	remerged, err := dependencies.Remerged(ctx, run.repository, names)
	return remerged, err == nil
}
