package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// The adoption rows, the plan line of the templates a profile leaves to
// another owner (A1), the line of the fixture that leaves them, and the
// install the plan names for a CI lane a profile keeps.
const (
	m5AdoptionCategory = "m5-adoption"
	m5Excluded         = "Deixados para a plataforma:"
	m5GithubDir        = ".github"
	m5CodeOwners       = "CODEOWNERS"
	m5ExcludeLine      = "templates: { exclude: [\".github/\"] }\n"
	m5CIInstall        = "execute golaw profile install " + m5Canonical + "@" + m5Version + " no CI"
)

func m5AdoptionCases() []m5Case {
	return []m5Case{
		{name: "init-profile-pins", run: (*suite).m5InitPins},
		{name: "init-profile-refresh", run: (*suite).m5InitRefresh},
		{name: "template-born-no-github-write", run: (*suite).m5TemplateBorn},
		{name: "init-profile-ci-line", run: (*suite).m5CILine},
		{name: "debt-seeded-once", run: (*suite).m5SeededOnce},
		{name: "debt-broadening-refused", run: (*suite).m5Broadening},
	}
}

// m5PinOf is the profile golaw.yml of a repository pins.
func m5PinOf(repo string) (policy.ProfileRef, error) {
	file, err := m5DecodePolicy(repo)
	if err != nil {
		return policy.ProfileRef{}, err
	}
	if file.Profile == nil {
		return policy.ProfileRef{}, fmt.Errorf("the policy of %s pins no profile: %w", repo, ErrAcceptance)
	}
	return *file.Profile, nil
}

// m5InitPins: the preview prints the pin and writes nothing; apply pins
// the stamp of the installed bundle and enrolls its digest (A3, D7).
func (s *suite) m5InitPins(ctx context.Context) error {
	source, err := s.m5Source(ctx)
	if err != nil {
		return err
	}
	if _, stepErr := s.m5Install(ctx, source, m5Tag); stepErr != nil {
		return stepErr
	}
	repo, err := s.repository(ctx, "pins", false)
	if err != nil {
		return err
	}
	if stepErr := writeFile(filepath.Join(repo, moduleFile), []byte(m5Module), fileMode); stepErr != nil {
		return stepErr
	}
	if stepErr := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); stepErr != nil {
		return stepErr
	}
	preview := s.m5Golaw(ctx, repo, commandInit, m5ProfileFlag, m5Name, m5CapabilitiesFlag, m5Capabilities)
	if stepErr := m5JudgePreview(preview, repo); stepErr != nil {
		return stepErr
	}
	if stepErr := success(s.m5Golaw(ctx, repo, commandInit, m5ProfileFlag, m5Name, m5CapabilitiesFlag, m5Capabilities, flagApply)); stepErr != nil {
		return stepErr
	}
	return s.m5JudgePinned(repo)
}

// m5JudgePreview: the preview succeeded, shows the pin and wrote nothing.
func m5JudgePreview(got result, repo string) error {
	if err := success(got); err != nil {
		return err
	}
	if !strings.Contains(got.stdout, "+profile: { name: "+m5Name+", source: "+m5Canonical+", version: "+m5Version) {
		return fmt.Errorf("the preview does not show the pin: %w", ErrAcceptance)
	}
	if _, err := os.Stat(filepath.Join(repo, policyFile)); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("the preview wrote golaw.yml: %w", ErrAcceptance)
	}
	return nil
}

// m5JudgePinned: the pin is the current stamp and the enrollment record
// carries its digest.
func (s *suite) m5JudgePinned(repo string) error {
	pin, err := m5PinOf(repo)
	if err != nil {
		return err
	}
	current, err := s.m5Current()
	if err != nil {
		return err
	}
	if pin != (policy.ProfileRef{Name: current.Name, Source: current.Source, Version: current.Version, Digest: current.Digest}) {
		return fmt.Errorf("the pin %+v is not the installed stamp %+v: %w", pin, current, ErrAcceptance)
	}
	dirs, err := s.m5Dirs()
	if err != nil {
		return err
	}
	record, found, err := policy.FindRecord(dirs.State, repo)
	if err != nil {
		return fmt.Errorf("read the enrollment record: %w", err)
	}
	if !found || record.ProfileDigest != pin.Digest {
		return fmt.Errorf("the enrollment record does not carry the pinned digest: %w", ErrAcceptance)
	}
	return nil
}

// m5InitRefresh: a newer install moves no pin; init --profile moves its
// version and digest alone, committed by the person (A19).
func (s *suite) m5InitRefresh(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "refresh-pin")
	if err != nil {
		return err
	}
	if _, stepErr := s.m5Install(ctx, setup.source, m5NewerTag); stepErr != nil {
		return stepErr
	}
	if stepErr := s.m5Unchanged(ctx, setup); stepErr != nil {
		return stepErr
	}
	if stepErr := success(s.m5Golaw(ctx, setup.repo, commandInit, m5ProfileFlag, m5Name, flagApply)); stepErr != nil {
		return stepErr
	}
	after, err := readFile(filepath.Join(setup.repo, policyFile))
	if err != nil {
		return err
	}
	if stepErr := m5OnlyPinMoved(setup.pin, after); stepErr != nil {
		return stepErr
	}
	if stepErr := s.gitSteps(ctx, setup.repo, []string{gitAdd, policyFile}, commitArgs("feat: move the fixture profile to 1.3.0")); stepErr != nil {
		return stepErr
	}
	// init enrolls a changed policy only once a commit holds it, so the
	// commit itself passes the armed rungs; the person then runs init
	// --apply again, as init asked.
	if stepErr := success(s.m5Golaw(ctx, setup.repo, commandInit, flagApply)); stepErr != nil {
		return stepErr
	}
	if stepErr := success(s.m5Golaw(ctx, setup.repo, commandScan, ".")); stepErr != nil {
		return stepErr
	}
	return s.m5ExpectProfile(ctx, setup.repo, m5Installed, "version "+m5NewerVersion)
}

// m5OnlyPinMoved requires the policy after a refresh to differ from the
// one before in the version and the digest of the pin alone.
func m5OnlyPinMoved(before, after []byte) error {
	was, err := policy.Decode(before)
	if err != nil {
		return fmt.Errorf("decode the policy before: %w", err)
	}
	now, err := policy.Decode(after)
	if err != nil {
		return fmt.Errorf("decode the policy after: %w", err)
	}
	if was.Profile == nil || now.Profile == nil || now.Profile.Name != was.Profile.Name || now.Profile.Source != was.Profile.Source ||
		now.Profile.Version == was.Profile.Version || now.Profile.Digest == was.Profile.Digest {
		return fmt.Errorf("the pin did not move its version and digest alone: %w", ErrAcceptance)
	}
	old, moved := strings.Split(string(before), "\n"), strings.Split(string(after), "\n")
	if len(old) != len(moved) {
		return fmt.Errorf("golaw.yml gained or lost lines: %w", ErrAcceptance)
	}
	changed := 0
	for i := range old {
		if old[i] != moved[i] {
			changed++
			if !strings.HasPrefix(moved[i], "profile:") {
				return fmt.Errorf("init moved %q: %w", moved[i], ErrAcceptance)
			}
		}
	}
	if changed != 1 {
		return fmt.Errorf("%d lines moved, want the profile line alone: %w", changed, ErrAcceptance)
	}
	return nil
}

// m5GithubFiles lists the regular files under .github/ by slash path.
func m5GithubFiles(repo string) ([]string, error) {
	base := filepath.Join(repo, m5GithubDir)
	var files []string
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || !entry.Type().IsRegular() {
			return walkErr
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return fmt.Errorf("relativize %s: %w", path, err)
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list .github/: %w", err)
	}
	slices.Sort(files)
	return files, nil
}

// m5DecodePolicy is the decoded golaw.yml of a repository.
func m5DecodePolicy(repo string) (policy.File, error) {
	data, err := readFile(filepath.Join(repo, policyFile))
	if err != nil {
		return policy.File{}, err
	}
	file, err := policy.Decode(data)
	if err != nil {
		return policy.File{}, fmt.Errorf("decode the policy of %s: %w", repo, err)
	}
	return file, nil
}

// m5CILine: a bundle that leaves no destination to the platform keeps the
// managed CI lane, whose make ci runs golaw scan, so the preview of init
// names the install that lane runs first (A19, Decision 4); the fixture,
// which leaves .github/ to the platform, names none (init-profile-pins).
func (s *suite) m5CILine(ctx context.Context) error {
	dirs, err := s.m5Dirs()
	if err != nil {
		return err
	}
	if stepErr := m5KeepingCI(s.root, dirs); stepErr != nil {
		return stepErr
	}
	repo, err := s.repository(ctx, "ci-line", false)
	if err != nil {
		return err
	}
	if stepErr := writeFile(filepath.Join(repo, moduleFile), []byte(m5Module), fileMode); stepErr != nil {
		return stepErr
	}
	if stepErr := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); stepErr != nil {
		return stepErr
	}
	got := s.m5Golaw(ctx, repo, commandInit, m5ProfileFlag, m5Name, m5CapabilitiesFlag, m5Capabilities)
	if stepErr := success(got); stepErr != nil {
		return stepErr
	}
	if !strings.Contains(got.stdout, m5CIInstall) {
		return fmt.Errorf("the preview names no install for the CI lane: %q: %w", got.stdout, ErrAcceptance)
	}
	return nil
}

// m5KeepingCI makes the current bundle of the fixture a copy whose
// rules.yml leaves no destination to the platform, stamped again.
func m5KeepingCI(root string, dirs policy.Dirs) error {
	dir := profile.Dir(dirs.Config, m5Name)
	if err := m5CopyBundle(root, dir); err != nil {
		return err
	}
	rules := filepath.Join(dir, profile.RulesFile)
	body, err := readFile(rules)
	if err != nil {
		return err
	}
	kept := strings.Replace(string(body), m5ExcludeLine, "", 1)
	if kept == string(body) {
		return fmt.Errorf("the fixture bundle leaves nothing to the platform: %w", ErrAcceptance)
	}
	if stepErr := writeFile(rules, []byte(kept), fileMode); stepErr != nil {
		return stepErr
	}
	_, err = m5Stamp(dir, m5Version, m5Date)
	return err
}
