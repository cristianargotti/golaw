package hook

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// input is what the engine judges: the candidates folded over the base for
// a write, the files a command or a post event changed for a post event,
// with the gaps of a detection that could not be measured, and the event
// alone else. A target over the read bound stays a target: the name rules
// judge it and the rules that need its bytes leave it unread.
func (run execution) input(ctx context.Context, dependencies Dependencies) (model.Input, []Gap, error) {
	event := run.request.Event
	var snapshot model.Snapshot = run.base
	var paths []string
	var gaps []Gap
	var err error
	switch event.Shape {
	case model.ShapeWrite:
		if denial := protectPolicy(run.repository, event.Candidates); denial != nil {
			return model.Input{}, nil, denial
		}
		snapshot, err = dependencies.Overlay(ctx, snapshot, event.Candidates, run.request.Content)
		paths = candidatePaths(event.Candidates)
	case model.ShapeRead, model.ShapeShell:
	case model.ShapePost:
		paths, gaps = run.postPaths(ctx, dependencies)
		if event.Post != nil {
			post := *event.Post
			post.Changed = paths
			event.Post = &post
		}
	}
	if err != nil {
		return model.Input{}, nil, fmt.Errorf("%w: prepare hook input: %w", model.ErrCannotJudge, err)
	}
	return model.Input{
		Mode: model.ModeHook, Policy: run.policy, Snapshot: snapshot, Targets: selectFiles(snapshot, paths), Event: &event,
	}, gaps, nil
}

// postPaths are the files a post event judges: those the host reports and
// those the content identity of a command found changed, with the gap of a
// detection that could not be measured and of each file it could not tell
// changed or not, which never block.
func (run execution) postPaths(ctx context.Context, dependencies Dependencies) ([]string, []Gap) {
	paths := slices.Clone(run.request.Hints)
	var gaps []Gap
	if run.request.Event.Tool == "Bash" || run.request.Event.Tool == "PowerShell" {
		changed, found, err := run.changedPaths(ctx, dependencies)
		if err != nil {
			gaps = append(gaps, detectionGap(err))
		}
		gaps = append(gaps, found...)
		paths = append(paths, changed...)
	}
	slices.Sort(paths)
	return slices.Compact(paths), gaps
}

// changedPaths are the paths whose content a command changed and, apart,
// the gaps of what it could not compare: a move of HEAD git could not
// answer, a path whose blob that move hides and a path a size alone
// identifies at both ends.
func (run execution) changedPaths(ctx context.Context, dependencies Dependencies) ([]string, []Gap, error) {
	if run.stateReason != "" {
		return nil, nil, fmt.Errorf("%w: %s", model.ErrCannotJudge, run.stateReason)
	}
	key, err := StateKey(run.repository.Identity, run.request.Event)
	if err != nil {
		return nil, nil, err
	}
	before, err := dependencies.State.Load(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	if before.Reason != "" {
		return nil, nil, fmt.Errorf("%w: %s", model.ErrCannotJudge, before.Reason)
	}
	moved := run.movedSince(ctx, dependencies, before.Head)
	also := append(slices.Sorted(maps.Keys(before.Files)), slices.Sorted(maps.Keys(moved.paths))...)
	after, changes, err := run.captureIdentities(ctx, dependencies, also)
	if err != nil {
		return nil, nil, err
	}
	var gaps []Gap
	if !moved.known {
		gaps = append(gaps, Gap{Cause: causeMove})
	}
	if dependencies.Changes != nil {
		gaps = append(gaps, pathGaps(sizedAtBothEnds(before.Files, after), causeReadBound)...)
	}
	changed, unknown := run.contentChanged(ctx, dependencies, before, after, changes, moved)
	return changed, append(gaps, unknown...), nil
}

// sizedAtBothEnds are the paths git status named that a size identifies
// before and after a command alike: a file over the read bound, whose
// bytes no identity holds, so a rewrite that keeps its size cannot be told
// from no change at all.
func sizedAtBothEnds(before, after map[string]string) []string {
	var sized []string
	for path, identity := range before {
		if strings.HasPrefix(identity, largeIdentity) && after[path] == identity {
			sized = append(sized, path)
		}
	}
	slices.Sort(sized)
	return sized
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
