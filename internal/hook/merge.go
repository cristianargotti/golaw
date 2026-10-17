package hook

import (
	"context"
	"slices"
	"strings"
)

// fromMerge leaves out of what a command changed what git wrote for it.
// A path git left unmerged that git status did not name before the command
// holds the conflict git wrote from both sides, so it is a gap until the
// event that writes its resolution judges it. When the command started a
// merge it left in progress, MERGE_HEAD present after it and not before, a
// path the merge staged with its bytes on disk, among those a commit the
// merge brings holds apart from HEAD, is named as brought by the merge, an
// automatic merge of both sides included, because none of its bytes is the
// command's; a path the command wrote after the merge, one the merge did
// not touch and one git status named before stay judged.
func (run execution) fromMerge(ctx context.Context, dependencies Dependencies, before Baseline, changes Changes, changed []string) ([]string, []Gap) {
	var unmerged, staged []string
	for _, path := range changed {
		if _, named := before.Files[path]; named {
			continue
		}
		switch {
		case changes.Unmerged[path]:
			unmerged = append(unmerged, path)
		case changes.Staged[path]:
			staged = append(staged, path)
		}
	}
	changed = slices.DeleteFunc(changed, among(unmerged))
	gaps := pathGaps(unmerged, causeUnmerged)
	if changes.Merge == "" || changes.Merge == before.Merge || len(staged) == 0 {
		return changed, gaps
	}
	merged, known := run.mergeBrought(ctx, dependencies, changes, staged)
	if !known {
		return slices.DeleteFunc(changed, among(staged)), append(gaps, pathGaps(staged, causeMergeUnknown)...)
	}
	return slices.DeleteFunc(changed, among(merged)), append(gaps, pathGaps(merged, causeMerged)...)
}

// mergeBrought are the staged paths a commit MERGE_HEAD names holds apart
// from HEAD, asked of git once for each such commit, one for any merge but
// an octopus; it is false when git cannot answer, or no binding asks it.
func (run execution) mergeBrought(ctx context.Context, dependencies Dependencies, changes Changes, staged []string) ([]string, bool) {
	if dependencies.Moved == nil {
		return nil, false
	}
	apart := map[string]bool{}
	for commit := range strings.FieldsSeq(changes.Merge) {
		paths, err := dependencies.Moved(ctx, run.repository, commit, changes.Head)
		if err != nil {
			return nil, false
		}
		for path := range paths {
			apart[path] = true
		}
	}
	return slices.DeleteFunc(slices.Clone(staged), func(path string) bool { return !apart[path] }), true
}
