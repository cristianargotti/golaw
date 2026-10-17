package main

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/source"
)

// automatic is the blob the merge git makes of two commits by itself and
// the blob the second holds at each path, in one git merge-tree and one git
// cat-file, the objects of the merge kept out of the repository.
func (h hookBinding) automatic(ctx context.Context, repo hook.Repository, ours, theirs string, paths []string) (map[string][]string, error) {
	found, err := h.cache.Discover(ctx, repo.Root)
	if err != nil {
		return nil, discoverFailed(repo.Root, err)
	}
	blobs, err := source.AutomaticMerge(ctx, found, ours, theirs, paths)
	if err != nil {
		return nil, fmt.Errorf("merge %s into %s in %s as git merges by itself: %w", theirs, ours, found.Root, err)
	}
	return blobs, nil
}

// remerged is, for each merge commit, the paths it holds apart from the
// merge git makes of its parents by itself, in one git log --remerge-diff.
func (h hookBinding) remerged(ctx context.Context, repo hook.Repository, merges []string) (map[string][]string, error) {
	found, err := h.cache.Discover(ctx, repo.Root)
	if err != nil {
		return nil, discoverFailed(repo.Root, err)
	}
	remerged, err := source.Remerged(ctx, found, merges)
	if err != nil {
		return nil, fmt.Errorf("remerge the merges of %s as git merges by itself: %w", found.Root, err)
	}
	return remerged, nil
}
