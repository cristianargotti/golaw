package main

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/model"
)

const (
	m3TemplateCategory = "m3-template"
	// m3ReplayOS is the one system the generated CI names as its runner: a
	// replay elsewhere would prove a pipeline nobody runs.
	m3ReplayOS = "linux"
	m3CIFile   = ".github/workflows/ci.yml"
	// The capabilities a combination, a violation and a capability row all
	// name: one spelling keeps the three catalogs on the same capability.
	m3CapabilityAPI   = "api"
	m3CapabilityImage = "image"
)

// m3Combinations are the capability sets of the replay proof of the CI
// template: go alone, go beside each capability that adds a job or a tool,
// and every capability at once, so each job the template can render runs.
func m3Combinations() map[string][]string {
	return map[string][]string{
		"every":        model.Capabilities(),
		"go":           {"go"},
		"go-api":       {m3CapabilityAPI, "go"},
		"go-db":        {"db", "go"},
		"go-hexagonal": {"go", "hexagonal"},
		"go-image":     {"go", m3CapabilityImage},
	}
}

func m3TemplateNames() []string {
	return slices.Sorted(maps.Keys(m3Combinations()))
}

// templateM3 adopts the service under one combination and replays the CI
// file init rendered for it, so the row judges the pipeline an adopter gets
// and never a copy of it. A file without a run step would pass vacuously,
// so it fails the row.
func (s *suite) templateM3(ctx context.Context, name string) error {
	capabilities, found := m3Combinations()[name]
	if !found {
		return fmt.Errorf("unknown M3 template combination %q: %w", name, ErrAcceptance)
	}
	repo, err := s.m3ServiceRepository(ctx, name, capabilities)
	if err != nil {
		return err
	}
	data, err := readFile(filepath.Join(repo, filepath.FromSlash(m3CIFile)))
	if err != nil {
		return err
	}
	steps, err := assets.RunSteps(data)
	if err != nil {
		return fmt.Errorf("read the run steps of %s: %w", m3CIFile, err)
	}
	if len(steps) == 0 {
		return fmt.Errorf("%s holds no run step: %w", m3CIFile, ErrAcceptance)
	}
	return s.m3Replay(ctx, repo, steps)
}
