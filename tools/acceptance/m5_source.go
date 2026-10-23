package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// The fixture bundle of the kit (D16) and the newer version the lifecycle
// rows install beside it.
const (
	m5Name         = "fixture"
	m5Version      = "1.2.3"
	m5Tag          = "v1.2.3"
	m5Date         = "2026-10-20"
	m5NewerVersion = "1.3.0"
	m5NewerTag     = "v1.3.0"
	m5NewerDate    = "2026-10-21"
	m5Canonical    = "https://example.test/profiles/fixture"
	m5BundleSource = "internal/profile/testdata/fixture"
	m5Onboarding   = "ONBOARDING.md"
	m5SourcesDir   = "sources"
)

// m5CopyBundle copies every regular file of the module's fixture bundle
// into dir, the construction profiletest.BareSource commits, because the
// tool imports no test package.
func m5CopyBundle(root, dir string) error {
	return m3CopyTree(filepath.Join(root, filepath.FromSlash(m5BundleSource)), dir)
}

// m5Stamp writes the stamp of a bundle directory for version and date,
// with the digest profile.Digest computes over its other files (D2), and
// answers the pin that names it.
func m5Stamp(dir, version, date string) (policy.ProfileRef, error) {
	digest, err := profile.Digest(dir)
	if err != nil {
		return policy.ProfileRef{}, fmt.Errorf("digest the bundle %s: %w", dir, err)
	}
	stamp := strings.Join([]string{
		"name: " + m5Name, "version: " + version, "date: " + date, "source: " + m5Canonical, "digest: " + digest, "",
	}, "\n")
	if stepErr := writeFile(filepath.Join(dir, policy.StampFile), []byte(stamp), fileMode); stepErr != nil {
		return policy.ProfileRef{}, stepErr
	}
	return policy.ProfileRef{Name: m5Name, Source: m5Canonical, Version: version, Digest: digest}, nil
}

// m5NewerBundle turns a copy of the fixture bundle into version 1.3.0:
// one line added to the onboarding page moves the digest, and the stamp
// names the new version, date and digest.
func m5NewerBundle(dir string) error {
	page := filepath.Join(dir, m5Onboarding)
	body, err := readFile(page)
	if err != nil {
		return err
	}
	if writeErr := writeFile(page, append(body, []byte("\nVersion 1.3.0 adds this line.\n")...), fileMode); writeErr != nil {
		return writeErr
	}
	_, err = m5Stamp(dir, m5NewerVersion, m5NewerDate)
	return err
}

// m5Source is a bare repository under the case's base holding the fixture
// bundle at m5Tag and the newer one at m5NewerTag; its directory path is
// the URL git clones, so no row reaches the network.
func (s *suite) m5Source(ctx context.Context) (string, error) {
	work := filepath.Join(s.base, m5SourcesDir, "work")
	bare := filepath.Join(s.base, m5SourcesDir, m5Name+".git")
	if err := m5CopyBundle(s.root, work); err != nil {
		return "", err
	}
	first := [][]string{{commandInit, m2Quiet}, {gitAdd, "."}, commitArgs("feat: the fixture bundle"), {"tag", m5Tag}}
	if err := s.gitSteps(ctx, work, first...); err != nil {
		return "", err
	}
	if err := m5NewerBundle(work); err != nil {
		return "", err
	}
	second := [][]string{
		{gitAdd, "."},
		commitArgs("feat: the fixture bundle at 1.3.0"),
		{"tag", m5NewerTag},
		{"clone", m2Quiet, "--bare", work, bare},
	}
	if err := s.gitSteps(ctx, work, second...); err != nil {
		return "", err
	}
	return bare, nil
}
