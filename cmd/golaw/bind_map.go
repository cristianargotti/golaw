package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genmap"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// readManifest answers the generation manifest of a snapshot, nil without
// an error when the repository has none, because the map is useful before
// the first golaw new.
func readManifest(s model.Snapshot) (*manifest.Manifest, error) {
	var found *manifest.Manifest
	data, err := s.Read(manifest.Path)
	if err == nil {
		var m manifest.Manifest
		m, err = manifest.Decode(data)
		found = &m
	}
	if err != nil && !errors.Is(err, model.ErrNotInSnapshot) {
		return nil, fmt.Errorf("read %s: %w", manifest.Path, err)
	}
	return found, nil
}

// repositoryMap builds the map of the worktree that holds root under the
// generate block of the policy in force.
func (b binding) repositoryMap(ctx context.Context, root string, pol *model.EffectivePolicy, area string) (genmap.Report, error) {
	report, err := b.buildMap(ctx, root, pol, area)
	if err != nil {
		return genmap.Report{}, fmt.Errorf("map %s: %w", root, err)
	}
	return report, nil
}

// buildMap reads the snapshot and the manifest the map is built from.
func (b binding) buildMap(ctx context.Context, root string, pol *model.EffectivePolicy, area string) (genmap.Report, error) {
	s, err := b.snapshot(ctx, root)
	if err != nil {
		return genmap.Report{}, err
	}
	m, err := readManifest(s)
	if err != nil {
		return genmap.Report{}, err
	}
	return genmap.Build(ctx, genmap.Input{Snapshot: s, Manifest: m, Policy: pol.Generate, Area: area}) //nolint:wrapcheck // repositoryMap wraps every error of the build with the root
}

// session is the map a starting session receives as context, empty when it
// cannot be read, because a session must start whatever the map can read.
func (h hookBinding) session(ctx context.Context, repository hook.Repository, pol *model.EffectivePolicy) string {
	report, err := h.repositoryMap(ctx, repository.Root, pol, "")
	if err != nil {
		return ""
	}
	return report.Text(genmap.DefaultBudget)
}
