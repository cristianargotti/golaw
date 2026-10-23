package main

import (
	"context"
	"path/filepath"
	"slices"
)

// m5ProxyDir is the local module proxy prepareCandidate writes under the
// base of the run.
const m5ProxyDir = "proxy"

// prepareM5 builds the M5 candidate at its version behind the local
// module proxy (D21), installs the pinned tools once, as the M3 rows do,
// and leaves the environment every case starts from.
func (s *suite) prepareM5(ctx context.Context) error {
	if err := s.prepareCandidate(ctx, m5CandidateVersion); err != nil {
		return err
	}
	parent := slices.Clone(s.env)
	repo, err := s.m3InstallTools(ctx)
	if err != nil {
		return err
	}
	proxy, err := m2FileURL(filepath.Join(s.base, m5ProxyDir))
	if err != nil {
		return err
	}
	env, err := s.m2DatabaseEnvironment(ctx, repo, prependPath(parent, s.m2.Bin), false)
	if err != nil {
		return err
	}
	s.m2.Overlay = m3Overlay(env, s.m2.Bin, proxy)
	s.env = withOverlay(env, s.m2.Overlay)
	return nil
}

// m5Scope is the isolated home of one M5 case: config and state
// directories of its own, so a bundle one row installs is no other row's.
func (s *suite) m5Scope(category, name string) *suite {
	return s.m3Scope(category + "-" + name)
}
