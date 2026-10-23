package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/profile"
)

// The subcommands that make an installed version current again.
const (
	m5RefreshSub  = "refresh"
	m5RollbackSub = "rollback"
)

// m5Unchanged requires the committed pin and a clean tree after a
// lifecycle step: no step of the lifecycle writes golaw.yml (A19, D7).
func (s *suite) m5Unchanged(ctx context.Context, setup m5Setup) error {
	data, err := readFile(filepath.Join(setup.repo, policyFile))
	if err != nil {
		return err
	}
	if !bytes.Equal(data, setup.pin) {
		return fmt.Errorf("golaw.yml moved: %w", ErrAcceptance)
	}
	status := s.execute(ctx, command{program: s.git, args: []string{gitStatus, gitPorcelain}, dir: setup.repo, env: s.env})
	if stepErr := success(status); stepErr != nil {
		return stepErr
	}
	if status.stdout != "" {
		return fmt.Errorf("the repository changed: %s: %w", strings.TrimSpace(status.stdout), ErrAcceptance)
	}
	return nil
}

// m5CurrentIs requires the current bundle at version.
func (s *suite) m5CurrentIs(version string) error {
	current, err := s.m5Current()
	if err != nil {
		return err
	}
	if current.Version != version {
		return fmt.Errorf("the current bundle is %s, want %s: %w", current.Version, version, ErrAcceptance)
	}
	return nil
}

// m5Printed requires a command that succeeded and printed line.
func m5Printed(got result, line string) error {
	if err := success(got); err != nil {
		return err
	}
	if !strings.Contains(got.stdout, line) {
		return fmt.Errorf("printed %q, want %q: %w", got.stdout, line, ErrAcceptance)
	}
	return nil
}

// m5RefreshRow: refresh is the way out of a damaged current bundle (D9).
func (s *suite) m5RefreshRow(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "refresh")
	if err != nil {
		return err
	}
	dirs, err := s.m5Dirs()
	if err != nil {
		return err
	}
	if stepErr := m5AppendRules(dirs); stepErr != nil {
		return stepErr
	}
	if stepErr := m5ScanRefusal(s.m5Golaw(ctx, setup.repo, commandScan, "."), m5DigestWord, m5InstallCommand); stepErr != nil {
		return stepErr
	}
	if stepErr := success(s.m5Profile(ctx, m5RefreshSub, m5Name)); stepErr != nil {
		return stepErr
	}
	if stepErr := success(s.m5Golaw(ctx, setup.repo, commandScan, ".")); stepErr != nil {
		return fmt.Errorf("the scan after refresh: %w", stepErr)
	}
	if stepErr := s.m5Unchanged(ctx, setup); stepErr != nil {
		return stepErr
	}
	return s.m5ExpectProfile(ctx, setup.repo, m5Installed, m5PinnedDetail)
}

// m5RollbackRow: a newer install moves no pin, and rollback makes the
// previous version current again (D3).
func (s *suite) m5RollbackRow(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "rollback")
	if err != nil {
		return err
	}
	if _, stepErr := s.m5Install(ctx, setup.source, m5NewerTag); stepErr != nil {
		return stepErr
	}
	for _, check := range []func() error{
		func() error { return s.m5CurrentIs(m5NewerVersion) },
		func() error { return s.m5Unchanged(ctx, setup) },
		func() error { return s.m5ExpectProfile(ctx, setup.repo, m5Installed, m5PinnedDetail) },
		func() error {
			return m5Printed(s.m5Profile(ctx, m5RollbackSub, m5Name), "Perfil "+m5Name+" voltou para "+m5Version)
		},
		func() error { return s.m5CurrentIs(m5Version) },
		func() error { return s.m5Unchanged(ctx, setup) },
	} {
		if stepErr := check(); stepErr != nil {
			return stepErr
		}
	}
	return success(s.m5Golaw(ctx, setup.repo, commandScan, "."))
}

// m5RemoveRow: remove takes the bundle and its record, and the repository
// then names the install command.
func (s *suite) m5RemoveRow(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "remove")
	if err != nil {
		return err
	}
	if stepErr := m5Printed(s.m5Profile(ctx, m5RemoveSub, m5Name), "Perfil "+m5Name+" removido"); stepErr != nil {
		return stepErr
	}
	dirs, err := s.m5Dirs()
	if err != nil {
		return err
	}
	if _, err = os.Stat(profile.Dir(dirs.Config, m5Name)); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("the current bundle remains: %w", ErrAcceptance)
	}
	_, found, err := profile.ReadRecord(dirs.State, m5Name)
	if err != nil {
		return fmt.Errorf("read the install record: %w", err)
	}
	if found {
		return fmt.Errorf("the install record remains: %w", ErrAcceptance)
	}
	if stepErr := s.m5Unchanged(ctx, setup); stepErr != nil {
		return stepErr
	}
	if stepErr := s.m5ExpectProfile(ctx, setup.repo, "profile-not-installed", m5InstallCommand); stepErr != nil {
		return stepErr
	}
	return m5ScanRefusal(s.m5Golaw(ctx, setup.repo, commandScan, "."), m5InstallCommand)
}

// m5PinUnchanged runs the whole lifecycle and requires the pin and the tree
// unchanged after every step.
func (s *suite) m5PinUnchanged(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "pin")
	if err != nil {
		return err
	}
	steps := [][2]string{
		{m5InstallSub, setup.source + "@" + m5NewerTag},
		{m5RollbackSub, m5Name},
		{m5RefreshSub, m5Name},
		{m5RemoveSub, m5Name},
		{m5InstallSub, setup.source + "@" + m5Tag},
	}
	for _, step := range steps {
		if stepErr := success(s.m5Profile(ctx, step[0], step[1])); stepErr != nil {
			return fmt.Errorf("golaw profile %s %s: %w", step[0], step[1], stepErr)
		}
		if stepErr := s.m5Unchanged(ctx, setup); stepErr != nil {
			return fmt.Errorf("after golaw profile %s: %w", step[0], stepErr)
		}
	}
	return s.m5ExpectProfile(ctx, setup.repo, m5Installed, m5PinnedDetail)
}
