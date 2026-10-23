package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// m5CatchAll is a CODEOWNERS that gives every path to the service team, so
// only a profile can leave .github/ out of init.
const m5CatchAll = "* @example-org/service-team\n"

// m5TemplateBorn adopts three template births: as the template ships it,
// with a catch-all owner under the profile, and the same core only.
func (s *suite) m5TemplateBorn(ctx context.Context) error {
	source, err := s.m5Source(ctx)
	if err != nil {
		return err
	}
	if _, stepErr := s.m5Install(ctx, source, m5Tag); stepErr != nil {
		return stepErr
	}
	owned, err := s.m3BirthRepository(ctx, "birth-owned")
	if err != nil {
		return err
	}
	if stepErr := success(s.m5Golaw(ctx, owned, commandInit, m5ProfileFlag, m5Name, flagApply)); stepErr != nil {
		return stepErr
	}
	if stepErr := m5NoGithubWrite(owned); stepErr != nil {
		return stepErr
	}
	plain, err := s.m5CatchAllBirth(ctx, "birth-catch-all")
	if err != nil {
		return err
	}
	if stepErr := m5JudgeExcluded(s.m5Golaw(ctx, plain, commandInit, m5ProfileFlag, m5Name, flagApply)); stepErr != nil {
		return stepErr
	}
	if stepErr := m5NoGithubWrite(plain); stepErr != nil {
		return stepErr
	}
	return s.m5CoreBirth(ctx)
}

// m5CoreBirth is the near miss: the catch-all birth core only gets the
// generated CI file, so the exclusion is the profile's doing (D14).
func (s *suite) m5CoreBirth(ctx context.Context) error {
	core, err := s.m5CatchAllBirth(ctx, "birth-core")
	if err != nil {
		return err
	}
	if stepErr := success(s.m5Golaw(ctx, core, commandInit, flagApply)); stepErr != nil {
		return stepErr
	}
	files, err := m5GithubFiles(core)
	if err != nil {
		return err
	}
	if !slices.Contains(files, "workflows/ci.yml") {
		return fmt.Errorf("core only, init wrote %q under .github/, want the CI file: %w", files, ErrAcceptance)
	}
	return nil
}

// m5CatchAllBirth is the birth fixture whose CODEOWNERS names the
// catch-all owner alone, committed.
func (s *suite) m5CatchAllBirth(ctx context.Context, name string) (string, error) {
	repo, err := s.m3BirthRepository(ctx, name)
	if err != nil {
		return "", err
	}
	if stepErr := writeFile(filepath.Join(repo, m5GithubDir, m5CodeOwners), []byte(m5CatchAll), fileMode); stepErr != nil {
		return "", stepErr
	}
	steps := [][]string{{gitAdd, "."}, commitArgs("chore: give the catch-all owner every path")}
	return repo, s.gitSteps(ctx, repo, steps...)
}

// m5NoGithubWrite: the only file under .github/ is the CODEOWNERS the birth
// holds, and golaw.yml manages no .github/ path.
func m5NoGithubWrite(repo string) error {
	files, err := m5GithubFiles(repo)
	if err != nil {
		return err
	}
	if !slices.Equal(files, []string{m5CodeOwners}) {
		return fmt.Errorf("init wrote %q under .github/: %w", files, ErrAcceptance)
	}
	file, err := m5DecodePolicy(repo)
	if err != nil {
		return err
	}
	for _, managed := range file.Managed {
		if strings.HasPrefix(managed.Path, m5GithubDir+"/") {
			return fmt.Errorf("golaw.yml manages %s: %w", managed.Path, ErrAcceptance)
		}
	}
	return nil
}

// m5JudgeExcluded: init succeeded and named both templates it left to
// another owner under the plan line of A1.
func m5JudgeExcluded(got result) error {
	if err := success(got); err != nil {
		return err
	}
	for _, fragment := range []string{m5Excluded, ".github/dependabot.yml", ".github/workflows/ci.yml"} {
		if !strings.Contains(got.stdout, fragment) {
			return fmt.Errorf("the plan does not name %q: %w", fragment, ErrAcceptance)
		}
	}
	return nil
}
