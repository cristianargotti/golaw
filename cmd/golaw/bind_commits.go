package main

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/source"
)

// commits are the commits between two HEADs, with the name, the
// committer, the second, whether it is a merge and the paths of each, in
// one question to git.
func (h hookBinding) commits(ctx context.Context, repo hook.Repository, since, now string, from int64) ([]hook.Commit, error) {
	found, err := h.cache.Discover(ctx, repo.Root)
	if err != nil {
		return nil, discoverFailed(repo.Root, err)
	}
	listed, err := source.Commits(ctx, found, since, now, from)
	if err != nil {
		return nil, fmt.Errorf("list the commits of %s since %s: %w", found.Root, since, err)
	}
	commits := make([]hook.Commit, 0, len(listed))
	for _, commit := range listed {
		commits = append(commits, hook.Commit{
			Name: commit.Name, Committer: commit.Committer, Time: commit.Time, Merge: commit.Merge, Paths: commit.Paths,
		})
	}
	return commits, nil
}

// committer is the identity git commits as in the repository, in one
// question to git.
func (h hookBinding) committer(ctx context.Context, repo hook.Repository) (string, error) {
	found, err := h.cache.Discover(ctx, repo.Root)
	if err != nil {
		return "", discoverFailed(repo.Root, err)
	}
	ident, err := source.Committer(ctx, found)
	if err != nil {
		return "", fmt.Errorf("name the committer of %s: %w", found.Root, err)
	}
	return ident, nil
}
