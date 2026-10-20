package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/source"
)

// affected reads the indexed graph so unstaged imports cannot change its answer.
func (b binding) affected(ctx context.Context, root string) (runner.AffectedSet, error) {
	repo, err := source.Discover(ctx, root)
	if err != nil {
		return runner.AffectedSet{}, discoverFailed(root, err)
	}
	index, err := source.NewIndex(ctx, repo, source.IndexOptions{})
	if err != nil {
		return runner.AffectedSet{}, fmt.Errorf("read staged changes: %w", err)
	}
	paths := affectedPaths(index.Changes())
	if len(paths) == 0 {
		return runner.AffectedSet{}, nil
	}
	return b.affectedTree(ctx, repo.Root, paths)
}

// affectedPaths keeps both sides of a move so old importers remain affected.
func affectedPaths(changes []model.Change) []string {
	paths := make([]string, 0, 2*len(changes))
	for _, change := range changes {
		for _, name := range []string{change.OldPath, change.NewPath} {
			if name != "" {
				paths = append(paths, name)
			}
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

func (b binding) affectedTree(ctx context.Context, root string, paths []string) (runner.AffectedSet, error) {
	tree, err := b.stagedTree(ctx, root)
	if err != nil {
		return runner.AffectedSet{}, err
	}
	var set runner.AffectedSet
	// Finish cleanup before returning so its error remains part of the result.
	func() {
		defer func() { err = errors.Join(err, tree.Close()) }()
		set, err = runner.Affected(ctx, tree.Dir, paths, runner.UserEnv(b.environ()))
	}()
	if err != nil {
		return runner.AffectedSet{}, fmt.Errorf("measure staged packages: %w", err)
	}
	return set, nil
}

func (b binding) stagedTree(ctx context.Context, root string) (*runner.Tree, error) {
	dirs, err := b.dirs()
	if err != nil {
		return nil, err
	}
	parent := filepath.Join(dirs.State, "runner", "trees")
	if sweepErr := runner.Sweep(parent); sweepErr != nil {
		return nil, fmt.Errorf("sweep staged trees: %w", sweepErr)
	}
	tree, err := runner.Materialize(ctx, runner.MaterializeInput{
		Dir: root, Env: b.environ(), Parent: parent, Bounds: runner.DefaultBounds(),
	})
	if err != nil {
		return nil, fmt.Errorf("materialize staged tree: %w", err)
	}
	return tree, nil
}
