package stage

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
)

// Brought answers, while a merge is in progress (index.MergeHead not empty),
// the staged paths whose blob is the one MERGE_HEAD holds or the one the
// merge git makes by itself holds there, with no strategy option, with -X
// ours or with -X theirs (source.AutomaticMergeWith, three questions asked
// only while a merge is in progress, because the rung cannot see the
// options the merge was started with), so a rung never feeds back what git
// merged from the other side as the commit's own finding (section 8.1
// applied to the rungs); it answers nothing when no merge is in progress.
// It reads the changes of the index as taken against HEAD, the side the
// merge is made on.
func Brought(ctx context.Context, repo source.Repo, index *source.Index) ([]string, error) {
	theirs := index.MergeHead()
	if theirs == "" || repo.Head == "" {
		return nil, nil
	}
	staged := map[string]string{}
	for _, c := range index.Changes() {
		if c.NewPath != "" {
			staged[c.NewPath] = c.After
		}
	}
	paths := slices.Sorted(maps.Keys(staged))
	brought := map[string]bool{}
	for _, options := range [][]string{nil, {"ours"}, {"theirs"}} {
		if len(brought) == len(paths) {
			break
		}
		blobs, err := source.AutomaticMergeWith(ctx, repo, repo.Head, theirs, options, paths)
		if err != nil {
			return nil, fmt.Errorf("compare the merge in progress in %s: %w", repo.Root, err)
		}
		for path, sides := range blobs {
			if slices.Contains(sides, staged[path]) {
				brought[path] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(brought)), nil
}

// mergeOf answers what a merge in progress brought and the other parent the
// index is also compared with, both named against HEAD. With a base the
// input names, which no rung passes, the scan compares with that commit
// alone, the merge included: a path the branch changed since then holds the
// blob of the merge too, and is its own.
func mergeOf(ctx context.Context, in Input, index *source.Index) ([]string, string, error) {
	if in.Index.Base != "" {
		return nil, "", nil
	}
	brought, err := Brought(ctx, in.Repo, index)
	if err != nil {
		return nil, "", err
	}
	return brought, index.MergeHead(), nil
}

// leftOut drops from the staged files and changes the paths a merge in
// progress brought, which the pipeline judges and the commit does not own.
func leftOut(in model.Input, brought []string) model.Input {
	took := func(path string) bool {
		_, found := slices.BinarySearch(brought, path)
		return found
	}
	in.Targets = slices.DeleteFunc(in.Targets, func(f model.File) bool { return took(f.Path) })
	in.Changes = slices.DeleteFunc(in.Changes, func(c model.Change) bool { return c.NewPath != "" && took(c.NewPath) })
	return in
}
