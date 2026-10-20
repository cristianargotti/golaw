package main

import (
	"context"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// pushRanges keeps plumbing variables so linked worktrees resolve their own refs.
func (b binding) pushRanges(
	ctx context.Context, dir, remote string, input io.Reader,
) ([]runner.PushRange, error) {
	repo, err := source.Discover(ctx, dir)
	if err != nil {
		return nil, discoverFailed(dir, err)
	}
	updates, err := runner.ParseRefUpdates(input)
	if err != nil {
		return nil, fmt.Errorf("read push updates: %w", err)
	}
	ranges, err := runner.Ranges(ctx, repo.Root, remote, updates, b.environ())
	if err != nil {
		return nil, fmt.Errorf("resolve push ranges: %w", err)
	}
	return ranges, nil
}
