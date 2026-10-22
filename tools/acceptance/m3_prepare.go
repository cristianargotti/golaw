package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

const (
	m3CandidateVersion = "0.3.0"
	m3Preparation      = "m3-preparation"
	m3Verified         = "verified against the M3 candidate"
	m3ToolsCase        = "m3-tools"
	m3MakeName         = "make"
)

// prepareM3 builds the M3 candidate behind the local module proxy, installs
// the pinned toolchain through golaw setup in a repository of its own, and
// leaves the environment every M3 row starts from: the tools first on PATH,
// the candidate's proxy ahead of the public one and the fixture
// vulnerability database behind govulncheck, so no row reads a live
// advisory and the rows judge the candidate, never a published kit.
func (s *suite) prepareM3(ctx context.Context) error {
	if err := s.prepareCandidate(ctx, m3CandidateVersion); err != nil {
		return err
	}
	parent := slices.Clone(s.env)
	maker, err := s.lookup(m3MakeName)
	if err != nil {
		return fmt.Errorf("locate make: %w", err)
	}
	repo, err := s.m3InstallTools(ctx)
	if err != nil {
		return err
	}
	proxy, err := m2FileURL(filepath.Join(s.base, "proxy"))
	if err != nil {
		return err
	}
	env, err := s.m2DatabaseEnvironment(ctx, repo, prependPath(parent, s.m2.Bin), false)
	if err != nil {
		return err
	}
	s.m2.Make, s.m2.Overlay = maker, m3Overlay(env, s.m2.Bin, proxy)
	s.env = withOverlay(env, s.m2.Overlay)
	return nil
}

// m3InstallTools arms a repository of its own with golaw setup, the one
// place a row installs the pinned tools, and verifies every pin. The
// setup-clean receipt check of M2 is not reused because it names the M2
// version.
func (s *suite) m3InstallTools(ctx context.Context) (string, error) {
	repo, err := s.m3ToolsRepository(ctx)
	if err != nil {
		return "", err
	}
	env, err := s.m2PrerequisiteEnvironment(s.env, s.m2.Bin)
	if err != nil {
		return "", err
	}
	if setupErr := s.m2RunSetup(ctx, repo, env); setupErr != nil {
		return "", setupErr
	}
	if verifyErr := s.m2VerifyTools(ctx, repo, env, s.m2.Bin); verifyErr != nil {
		return "", verifyErr
	}
	s.m2.Ready = true
	return repo, nil
}

// m3ToolsRepository is an initialized and committed repository, the state
// golaw setup expects before it arms the hooks.
func (s *suite) m3ToolsRepository(ctx context.Context) (string, error) {
	repo, err := s.repository(ctx, m3ToolsCase, false)
	if err != nil {
		return "", err
	}
	if writeErr := writeFile(filepath.Join(repo, moduleFile), []byte(m2Module), fileMode); writeErr != nil {
		return "", writeErr
	}
	if gitErr := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); gitErr != nil {
		return "", gitErr
	}
	if initErr := s.m2Initialize(ctx, repo); initErr != nil {
		return "", initErr
	}
	if commitErr := s.gitSteps(ctx, repo, []string{gitAdd, "."}, commitArgs("test: prepare the M3 tools")); commitErr != nil {
		return "", commitErr
	}
	return repo, nil
}

// m3Overlay is the environment every M3 case adds over its own home: the
// candidate's proxy ahead of the public one, the kit's module outside the
// checksum database the proxy cannot answer for, and the scanner variables
// of the prepared environment.
func m3Overlay(env []string, bin, proxy string) []string {
	return []string{
		"GOBIN=" + bin,
		"GOPROXY=" + proxy + ",https://proxy.golang.org,direct",
		"GONOSUMDB=" + m2ModulePath,
		"GOSUMDB=sum.golang.org",
		// go get and go mod tidy refuse the read-only module graph that
		// -mod=readonly of the isolated home imposes.
		"GOFLAGS=-modcacherw",
		m2RealScannerKey + "=" + environmentValue(env, m2RealScannerKey),
		m2DatabaseKey + "=" + environmentValue(env, m2DatabaseKey),
	}
}

// withOverlay replaces every variable the overlay names, whatever its
// case, so the overlay wins over the isolated home of a case.
func withOverlay(env, overlay []string) []string {
	out := slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return slices.ContainsFunc(overlay, func(item string) bool {
			name, _, _ := strings.Cut(item, "=")
			return strings.EqualFold(name, key)
		})
	})
	return append(out, overlay...)
}

// m3Scope is the isolated home of one M3 case with the overlay of the
// prepared environment laid over it.
func (s *suite) m3Scope(name string) *suite {
	scope := s.m2Scope(name)
	scope.env = withOverlay(scope.env, s.m2.Overlay)
	return scope
}
