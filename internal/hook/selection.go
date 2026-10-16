package hook

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

func (run execution) input(ctx context.Context, dependencies Dependencies) (model.Input, string, error) {
	event := run.request.Event
	var snapshot model.Snapshot = run.base
	var paths []string
	var unmeasured string
	var err error
	switch event.Shape {
	case model.ShapeWrite:
		if denial := protectPolicy(run.repository, event.Candidates); denial != nil {
			return model.Input{}, "", denial
		}
		snapshot, err = dependencies.Overlay(ctx, snapshot, event.Candidates, run.request.Content)
		paths = candidatePaths(event.Candidates)
	case model.ShapeRead, model.ShapeShell:
	case model.ShapePost:
		paths, unmeasured = run.postPaths(ctx, dependencies)
		if event.Post != nil {
			post := *event.Post
			post.Changed = paths
			event.Post = &post
		}
	}
	if err != nil {
		return model.Input{}, "", fmt.Errorf("%w: prepare hook input: %w", model.ErrCannotJudge, err)
	}
	return model.Input{
		Mode: model.ModeHook, Policy: run.policy, Snapshot: snapshot,
		Targets: selectFiles(snapshot, paths), Event: &event,
	}, unmeasured, nil
}

func (run execution) postPaths(ctx context.Context, dependencies Dependencies) ([]string, string) {
	paths := slices.Clone(run.request.Hints)
	if run.request.Event.Tool == "Bash" || run.request.Event.Tool == "PowerShell" {
		changed, err := run.changedPaths(ctx, dependencies)
		if err != nil && len(paths) == 0 {
			return nil, err.Error()
		}
		if err == nil {
			paths = append(paths, changed...)
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths), ""
}

func (run execution) changedPaths(ctx context.Context, dependencies Dependencies) ([]string, error) {
	if run.stateReason != "" {
		return nil, fmt.Errorf("%w: %s", model.ErrCannotJudge, run.stateReason)
	}
	key, err := StateKey(run.repository.Identity, run.request.Event)
	if err != nil {
		return nil, err
	}
	before, reason, err := dependencies.State.Load(ctx, key)
	if err != nil {
		return nil, err
	}
	if reason != "" {
		return nil, fmt.Errorf("%w: %s", model.ErrCannotJudge, reason)
	}
	after, err := run.captureIdentities(ctx, dependencies)
	if err != nil {
		return nil, err
	}
	return Changed(before, after), nil
}

func protectPolicy(repository Repository, candidates []model.Candidate) error {
	for _, candidate := range candidates {
		for _, path := range []string{candidate.Path, candidate.From} {
			match := path == policyFile
			if repository.CaseInsensitive {
				match = strings.EqualFold(path, policyFile)
			}
			if match {
				return fmt.Errorf("%w: agent edits to golaw.yml are denied", model.ErrPolicy)
			}
		}
	}
	return nil
}

func candidatePaths(candidates []model.Candidate) []string {
	paths := make([]string, 0, len(candidates)*2)
	for _, candidate := range candidates {
		paths = append(paths, candidate.Path)
		if candidate.From != "" {
			paths = append(paths, candidate.From)
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

// selectFiles asks the snapshot for the files of the paths only, none
// when no path is named, so an event without targets lists no worktree.
func selectFiles(snapshot model.Snapshot, paths []string) []model.File {
	if len(paths) == 0 {
		return nil
	}
	return model.Select(snapshot, among(paths))
}

// among matches the paths of a list.
func among(paths []string) func(string) bool {
	set := make(map[string]bool, len(paths))
	for _, path := range paths {
		set[path] = true
	}
	return func(path string) bool { return set[path] }
}
