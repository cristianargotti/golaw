package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/surface"
)

// explainSurface answers the surface lines config explain prints after the
// policy. An app built without the production binding, as the fakes of a
// command test are, has no tree to read and prints the policy alone;
// newApp always binds one.
func (a *app) explainSurface(ctx context.Context, root string, now time.Time) ([]string, error) {
	if a.bind == nil {
		return nil, nil
	}
	return a.bind().surfaceExplain(ctx, root, now)
}

// surfaceExplain reads the policy in force and the worktree of root: the
// analyzer scope with its source and lists, the languages the snapshot
// holds, a line per file the inventory could not judge, and the reason the
// root's shape is outside M6 when it is.
func (b binding) surfaceExplain(ctx context.Context, root string, now time.Time) ([]string, error) {
	l, err := b.loaded(ctx, root, now)
	if err != nil {
		return nil, err
	}
	snapshot, err := b.snapshot(ctx, root)
	if err != nil {
		return nil, err
	}
	scope, err := surface.ScopeOf(ctx, snapshot, l.Policy)
	if err != nil {
		return nil, fmt.Errorf(scopeErrorFormat, root, err)
	}
	inventory, err := surface.Take(ctx, snapshot)
	if err != nil {
		return nil, fmt.Errorf("take the surface inventory of %s: %w", root, err)
	}
	lines := scope.Lines()
	lines = append(lines, computedLine("surface.languages", languageList(inventory.Languages)))
	for _, cause := range inventory.Unmeasured {
		lines = append(lines, computedLine("surface.unmeasured", cause))
	}
	if reason, outside := surface.OutsideShape(snapshot); outside {
		lines = append(lines, computedLine("surface.shape", reason))
	}
	return lines, nil
}

// computedLine renders a line whose value golaw computed from the tree.
func computedLine(key, value string) string {
	return key + " = " + value + " (" + policy.SourceComputed + ")"
}

// languageList renders languages as config explain renders a list.
func languageList(languages []surface.Language) string {
	names := make([]string, 0, len(languages))
	for _, l := range languages {
		names = append(names, string(l))
	}
	return "[" + strings.Join(names, ", ") + "]"
}
