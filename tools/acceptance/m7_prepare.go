package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
)

// The flag of setup that builds the pinned generators, the directory of the
// repository whose tools module pins them, and the capabilities it adopts so
// init renders the sqlc, oapi-codegen and goverter lines of that module.
const (
	m7SetupGen     = "--gen"
	m7ToolsRepo    = "gen-tools"
	m7Capabilities = "go,api,db,hexagonal"
)

// prepareM7 builds the M7 candidate at its version behind the local module
// proxy, installs the pinned Go tools once, as the M3 to M6 rows do, and
// leaves the environment every case starts from.
func (s *suite) prepareM7(ctx context.Context) error {
	if err := s.prepareCandidate(ctx, m7CandidateVersion); err != nil {
		return err
	}
	maker, err := s.lookup(m3MakeName)
	if err != nil {
		return fmt.Errorf("locate make: %w", err)
	}
	repo, err := s.m3InstallTools(ctx)
	if err != nil {
		return err
	}
	proxy, err := m2FileURL(filepath.Join(s.base, m5ProxyDir))
	if err != nil {
		return err
	}
	env, err := s.m2DatabaseEnvironment(ctx, repo, prependPath(slices.Clone(s.env), s.m2.Bin), false)
	if err != nil {
		return err
	}
	s.m2.Make, s.m2.Overlay = maker, m3Overlay(env, s.m2.Bin, proxy)
	s.env = withOverlay(env, s.m2.Overlay)
	return nil
}

// m7GenEnv is the scope's environment over the shared data home: the
// generator rows and the hook calls that run a generator read the receipts
// setup --gen wrote there.
func (s *m7Suite) m7GenEnv() []string {
	return m6WithData(s.env, s.shared.data)
}

// m7Generators builds the pinned generators once per run and answers the
// cause of a failed build to every later row without trying again.
func (s *m7Suite) m7Generators(ctx context.Context) error {
	if !s.gen.tried {
		s.gen.tried = true
		s.gen.err = s.m7BuildGenerators(ctx)
	}
	return s.gen.err
}

// m7BuildGenerators adopts a repository of its own and runs golaw setup
// --gen in it, the one place an M7 row reaches the module proxy beyond the
// preparation, as the M6 tool install is.
func (s *m7Suite) m7BuildGenerators(ctx context.Context) error {
	repo := filepath.Join(s.shared.base, m7ToolsRepo)
	if err := writeFile(filepath.Join(repo, moduleFile), []byte(m5Module), fileMode); err != nil {
		return err
	}
	if err := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); err != nil {
		return err
	}
	if err := success(s.m5Golaw(ctx, repo, commandInit, m5CapabilitiesFlag, m7Capabilities, flagApply)); err != nil {
		return fmt.Errorf("adopt the generator tools repository: %w", err)
	}
	if err := s.m7Tidy(ctx, filepath.Join(repo, m7ToolsDir)); err != nil {
		return err
	}
	env, err := s.m2PrerequisiteEnvironment(s.m7GenEnv(), s.m2.Bin)
	if err != nil {
		return err
	}
	got := s.execute(ctx, command{program: s.binary, args: []string{m6SetupCommand, m7SetupGen}, dir: repo, env: env})
	if err := success(got); err != nil {
		return fmt.Errorf("build the pinned generators: %w", err)
	}
	return nil
}
