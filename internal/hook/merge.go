package hook

import (
	"context"
	"slices"
	"strings"
)

// fromMerge leaves out of what a command changed what git wrote for it.
// A path git left unmerged that git status did not name before the command
// holds the conflict git wrote from both sides, so it is a gap until the
// event that writes its resolution judges it, unless the command started
// the merge and wrote bytes of its own there (resolutions). When the
// command started a merge it left in progress, MERGE_HEAD present after it
// and not before, a
// path staged with its bytes on disk whose staged blob is the one the merge
// git makes by itself holds there, with the strategy options the command
// text gave when the merge without them leaves it, or the one the merged
// commit holds, is named as brought by the merge, because none of its
// bytes is the command's, unless a commit the command created before the
// merge changed it (notOwn); a path whose staged blob is neither, the
// command's resolution or its own edit, a file only the local side changed
// included, one the command wrote after the merge and one git status named
// before stay judged.
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
	written, gaps := run.resolutions(ctx, dependencies, before, changes, unmerged)
	changed = slices.DeleteFunc(changed, func(path string) bool { return slices.Contains(unmerged, path) && !slices.Contains(written, path) })
	if changes.Merge == "" || changes.Merge == before.Merge || len(staged) == 0 {
		return changed, gaps
	}
	merged, unknown := run.mergeBrought(ctx, dependencies, changes, staged, before.Options)
	merged, untold := run.notOwn(ctx, dependencies, before, changes, merged)
	changed = slices.DeleteFunc(changed, among(slices.Concat(merged, unknown, untold)))
	return changed, slices.Concat(gaps, pathGaps(unknown, causeMergeUnknown), pathGaps(untold, causeCommits), pathGaps(merged, causeMerged))
}

// mergeBrought are the staged paths whose staged blob is one the merge git
// makes of HEAD and the commit MERGE_HEAD names by itself holds there, or
// that commit holds, asked of git once, and, when the command text gave
// strategy options, of the paths that merge leaves, those whose staged blob
// the merge with the options holds, asked once more. Apart, the paths it
// could not tell: every staged path for an octopus merge, which git
// merge-tree cannot make, when git cannot answer or no binding asks it, and
// the paths the first merge leaves when the merge with the options cannot
// be made.
func (run execution) mergeBrought(ctx context.Context, dependencies Dependencies, changes Changes, staged, options []string) ([]string, []string) {
	theirs := strings.Fields(changes.Merge)
	if len(theirs) != 1 {
		return nil, staged
	}
	merged, known := run.gitMerged(ctx, dependencies, changes, theirs[0], nil, staged)
	if !known {
		return nil, staged
	}
	rest := slices.DeleteFunc(slices.Clone(staged), among(merged))
	if len(options) == 0 || len(rest) == 0 {
		return merged, nil
	}
	more, known := run.gitMerged(ctx, dependencies, changes, theirs[0], options, rest)
	if !known {
		return merged, rest
	}
	merged = append(merged, more...)
	slices.Sort(merged)
	return merged, nil
}

// gitMerged are the staged paths whose staged blob is the one the merge git
// makes of HEAD and theirs with the strategy options holds there, or the one
// theirs holds, asked of git once through AutomaticWith, or through
// Automatic when only it is bound and no option is asked; it is false when
// git cannot answer or no binding asks it.
func (run execution) gitMerged(ctx context.Context, dependencies Dependencies, changes Changes, theirs string, options, staged []string) ([]string, bool) {
	var blobs map[string][]string
	var err error
	switch {
	case dependencies.AutomaticWith != nil:
		blobs, err = dependencies.AutomaticWith(ctx, run.repository, changes.Head, theirs, options, staged)
	case dependencies.Automatic != nil && len(options) == 0:
		blobs, err = dependencies.Automatic(ctx, run.repository, changes.Head, theirs, staged)
	default:
		return nil, false
	}
	if err != nil {
		return nil, false
	}
	return slices.DeleteFunc(slices.Clone(staged), func(path string) bool {
		blob := changes.Index[path]
		return blob == "" || !slices.Contains(blobs[path], blob)
	}), true
}
