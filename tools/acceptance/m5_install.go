package main

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// The lifecycle rows, the doctor code of a bundle in force and the
// fragment of its detail that names the pinned version.
const (
	m5InstallCategory = "m5-install"
	m5Installed       = "profile-installed"
	m5PinnedDetail    = "version " + m5Version
)

func m5InstallCases() []m5Case {
	return []m5Case{
		{name: m5InstallSub, run: (*suite).m5InstallRow},
		{name: "install-idempotent", run: (*suite).m5Idempotent},
		{name: "install-mirror", run: (*suite).m5Mirror},
		{name: "refresh", run: (*suite).m5RefreshRow},
		{name: "rollback", run: (*suite).m5RollbackRow},
		{name: "remove", run: (*suite).m5RemoveRow},
		{name: "pin-unchanged", run: (*suite).m5PinUnchanged},
	}
}

// m5FixtureDigest is the digest the module's fixture stamp carries.
func m5FixtureDigest(root string) (string, error) {
	stamp, err := policy.LoadStamp(filepath.Join(root, filepath.FromSlash(m5BundleSource), policy.StampFile))
	if err != nil {
		return "", fmt.Errorf("read the fixture stamp: %w", err)
	}
	return stamp.Digest, nil
}

// m5InstallRow: the install printed the version, the source and tag given
// and the digest, the current stamp is the fixture's, and doctor reports it.
func (s *suite) m5InstallRow(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, m5InstallSub)
	if err != nil {
		return err
	}
	digest, err := m5FixtureDigest(s.root)
	if err != nil {
		return err
	}
	line := "Perfil " + m5Name + " " + m5Version + " instalado de " + setup.source + "@" + m5Tag + ", digest " + digest
	if !strings.Contains(setup.installed.stdout, line) {
		return fmt.Errorf("the install printed %q, want %q: %w", setup.installed.stdout, line, ErrAcceptance)
	}
	current, err := s.m5Current()
	if err != nil {
		return err
	}
	if current.Version != m5Version || current.Digest != digest || current.Source != m5Canonical || current.Date != m5Date {
		return fmt.Errorf("the current stamp is %+v, want the fixture's: %w", current, ErrAcceptance)
	}
	return s.m5ExpectProfile(ctx, setup.repo, m5Installed, m5PinnedDetail)
}

// m5Idempotent: a second install of the same tag copies nothing.
func (s *suite) m5Idempotent(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "idempotent")
	if err != nil {
		return err
	}
	dirs, err := s.m5Dirs()
	if err != nil {
		return err
	}
	before, err := treeDigests(dirs.Config)
	if err != nil {
		return err
	}
	got, err := s.m5Install(ctx, setup.source, m5Tag)
	if err != nil {
		return err
	}
	if !strings.Contains(got.stdout, "Perfil "+m5Name+" "+m5Version+" já estava instalado") {
		return fmt.Errorf("the second install printed %q: %w", got.stdout, ErrAcceptance)
	}
	after, err := treeDigests(dirs.Config)
	if err != nil {
		return err
	}
	if !maps.Equal(before, after) {
		return fmt.Errorf("the second install changed the config directory: %w", ErrAcceptance)
	}
	if stepErr := s.m5Unchanged(ctx, setup); stepErr != nil {
		return stepErr
	}
	return s.m5ExpectProfile(ctx, setup.repo, m5Installed, m5PinnedDetail)
}

// m5Mirror: the local source is a mirror, so the canonical source is named,
// pinned and recorded beside the URL given (D9).
func (s *suite) m5Mirror(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "mirror")
	if err != nil {
		return err
	}
	if !strings.Contains(setup.installed.stdout, m5Canonical) {
		return fmt.Errorf("the install from a mirror did not name %s: %w", m5Canonical, ErrAcceptance)
	}
	pin, err := policy.Decode(setup.pin)
	if err != nil || pin.Profile == nil || pin.Profile.Source != m5Canonical {
		return fmt.Errorf("the pin does not name the canonical source: %w", ErrAcceptance)
	}
	dirs, err := s.m5Dirs()
	if err != nil {
		return err
	}
	record, found, err := profile.ReadRecord(dirs.State, m5Name)
	if err != nil {
		return fmt.Errorf("read the install record: %w", err)
	}
	if !found || len(record.Versions) == 0 {
		return fmt.Errorf("the install record is absent: %w", ErrAcceptance)
	}
	last := record.Versions[len(record.Versions)-1]
	if last.URL != setup.source || last.Source != m5Canonical || last.Tag != m5Tag {
		return fmt.Errorf("the record holds %+v: %w", last, ErrAcceptance)
	}
	return s.m5ExpectProfile(ctx, setup.repo, m5Installed, m5PinnedDetail)
}
