package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/cristianargotti/golaw/internal/genmap"
	"github.com/cristianargotti/golaw/internal/model"
)

// areaContext builds the context of one area of the worktree that holds
// root; for an unknown area it answers genmap.ErrNoArea with the areas
// the layout places a source in.
func (b binding) areaContext(ctx context.Context, root string, pol *model.EffectivePolicy, area string) (genmap.Context, []string, error) {
	c, areas, err := b.buildContext(ctx, root, pol, area)
	if err != nil {
		return genmap.Context{}, areas, fmt.Errorf("context %s: %w", root, err)
	}
	return c, nil, nil
}

// buildContext reads the snapshot and the manifest the context is built
// from.
func (b binding) buildContext(ctx context.Context, root string, pol *model.EffectivePolicy, area string) (genmap.Context, []string, error) {
	s, err := b.snapshot(ctx, root)
	if err != nil {
		return genmap.Context{}, nil, err
	}
	m, err := readManifest(s)
	if err != nil {
		return genmap.Context{}, nil, err
	}
	in := genmap.Input{Snapshot: s, Manifest: m, Policy: pol.Generate, Area: area}
	c, err := genmap.BuildContext(ctx, in)
	if errors.Is(err, genmap.ErrNoArea) {
		return genmap.Context{}, genmap.Areas(in), err
	}
	return c, nil, err //nolint:wrapcheck // areaContext wraps every error with the root
}
