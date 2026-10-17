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
	created, merges, ok := run.created(ctx, dependencies, before)
	if !ok {
		return slices.DeleteFunc(changed, among(brought)), pathGaps(brought, causeCommits)
	}
	unknown := among(run.resolved(ctx, dependencies, merges, brought, created))
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
// records whole seconds, by the identity git commits as now. It is false
// when git cannot answer the commits or the identity.
func (run execution) created(ctx context.Context, dependencies Dependencies, before Baseline) (map[string]bool, []Commit, bool) {
	if dependencies.Commits == nil || dependencies.Committer == nil {
		return nil, nil, false
	}
	committer, err := dependencies.Committer(ctx, run.repository)
	if err != nil {
		return nil, nil, false
	}
	from := before.At.Unix()
	commits, err := dependencies.Commits(ctx, run.repository, before.Head, run.repository.Head, from)
	if err != nil {
		return nil, nil, false
	}
	paths := map[string]bool{}
	var merges []Commit
	for _, commit := range commits {
		switch {
		case commit.Time < from || commit.Committer != committer:
		case commit.Merge:
			merges = append(merges, commit)
		default:
			for _, path := range commit.Paths {
				paths[path] = true
			}
		}
	}
	return paths, merges, true
}

// resolved adds to created the paths the move brought that a merge of the
// command holds apart both from every parent, as git log names them, and
// from the merge git makes of its parents by itself, asked of git once for
// every such merge and only when one holds such a path, and answers the
// paths it could not tell, every such path of a merge when git cannot make
// it by itself or no binding asks it.
func (run execution) resolved(ctx context.Context, dependencies Dependencies, merges []Commit, brought []string, created map[string]bool) []string {
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
	if len(names) == 0 {
		return nil
	}
	remerged, known := run.remerged(ctx, dependencies, names)
	var unknown []string
	for _, name := range names {
		for _, path := range apart[name] {
			switch {
			case !known:
				unknown = append(unknown, path)
			case slices.Contains(remerged[name], path):
				created[path] = true
			}
		}
	}
	return unknown
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
