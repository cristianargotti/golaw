package rung

import (
	"context"
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/stage"
)

// RunCommitMsg is the commit-msg rung (A21): the commits family over the
// message file git hands the hook, read as git will record it, and over the
// staged changes of the commit being made, without what a merge in
// progress brought, which the pipeline judges.
func RunCommitMsg(ctx context.Context, deps Dependencies, file string) Outcome {
	a, out, ok := gate(ctx, deps, CommitMsg)
	if !ok {
		return out
	}
	language := a.loaded.Policy.Language
	in, err := commitInput(ctx, deps, a.loaded.Policy, file)
	if err != nil {
		return failed(CommitMsg, language, err)
	}
	run, err := engine.Dispatch(ctx, deps.Registry, in)
	if err != nil {
		return failed(CommitMsg, language, fmt.Errorf("judge the commit: %w", err))
	}
	return judged(CommitMsg, language, run)
}

// commitInput is the commit mode input of contract section 2.1: the policy
// in force, the index git commits through, its staged files and changes
// without what a merge in progress brought, and the commit.
func commitInput(ctx context.Context, deps Dependencies, p *model.EffectivePolicy, file string) (model.Input, error) {
	commit, err := readCommit(ctx, deps, file)
	if err != nil {
		return model.Input{}, err
	}
	repo, err := source.Discover(ctx, deps.Dir)
	if err != nil {
		return model.Input{}, fmt.Errorf("discover %s: %w", deps.Dir, err)
	}
	index, err := source.NewIndex(ctx, repo, source.IndexOptions{IndexFile: indexFile(deps)})
	if err != nil {
		return model.Input{}, fmt.Errorf("freeze the index: %w", err)
	}
	brought, err := stage.Brought(ctx, repo, index)
	if err != nil {
		return model.Input{}, fmt.Errorf("compare the merge in progress: %w", err)
	}
	in := model.Input{
		Mode: model.ModeCommit, Policy: p, Snapshot: index,
		Targets: index.Staged(), Changes: index.Changes(), Commit: &commit,
	}
	return leaveOut(in, brought), nil
}

// leaveOut drops from the staged files and changes the paths a merge in
// progress brought, as the staged scan does.
func leaveOut(in model.Input, brought []string) model.Input {
	took := func(path string) bool { return slices.Contains(brought, path) }
	in.Targets = slices.DeleteFunc(in.Targets, func(f model.File) bool { return took(f.Path) })
	in.Changes = slices.DeleteFunc(in.Changes, func(c model.Change) bool { return c.NewPath != "" && took(c.NewPath) })
	return in
}
