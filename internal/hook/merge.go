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
// path staged with its bytes on disk whose staged blob is the one the merge
// git makes by itself holds there, or the one the merged commit holds, is
// named as brought by the merge, because none of its bytes is the
// command's; a path whose staged blob is neither, the command's resolution
// or its own edit, a file only the local side changed included, one the
// command wrote after the merge and one git status named before stay
// judged.
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

// mergeBrought are the staged paths whose staged blob is one the merge git
// makes of HEAD and the commit MERGE_HEAD names by itself holds there, or
// that commit holds, asked of git once; it is false for an octopus merge,
// which git merge-tree cannot make, when git cannot answer, or when no
// binding asks it.
func (run execution) mergeBrought(ctx context.Context, dependencies Dependencies, changes Changes, staged []string) ([]string, bool) {
	theirs := strings.Fields(changes.Merge)
	if dependencies.Automatic == nil || len(theirs) != 1 {
		return nil, false
	}
	blobs, err := dependencies.Automatic(ctx, run.repository, changes.Head, theirs[0], staged)
	if err != nil {
		return nil, false
	}
	return slices.DeleteFunc(slices.Clone(staged), func(path string) bool {
		blob := changes.Index[path]
		return blob == "" || !slices.Contains(blobs[path], blob)
	}), true
}
