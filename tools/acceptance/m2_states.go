package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
)

func (s *suite) m2StagedRepository(ctx context.Context, name string) (string, error) {
	repo, err := s.repository(ctx, name, false)
	if err != nil {
		return "", err
	}
	files := m2StagedFiles(name)
	if writeErr := materialize(repo, files.head); writeErr != nil {
		return "", writeErr
	}
	if initErr := s.gitSteps(ctx, repo, []string{commandInit, "--quiet"}); initErr != nil {
		return "", initErr
	}
	if initErr := s.m2Initialize(ctx, repo); initErr != nil {
		return "", initErr
	}
	if commitErr := s.gitSteps(ctx, repo, []string{gitAdd, "."}, commitArgs("test: prepare staged fixture")); commitErr != nil {
		return "", commitErr
	}
	if strings.HasPrefix(name, "alternate-") {
		repo, err = s.m2AlternateRepository(ctx, repo, name)
	}
	if err != nil {
		return "", err
	}
	if err := s.m2PopulateState(ctx, repo, name, files); err != nil {
		return "", err
	}
	return repo, nil
}

func (s *suite) m2Initialize(ctx context.Context, repo string) error {
	return success(s.execute(ctx, command{
		program: s.binary, args: []string{commandInit, flagApply}, dir: repo, env: s.env,
	}))
}

func (s *suite) m2PopulateState(ctx context.Context, repo, name string, files m2FixtureFiles) error {
	family, _, _ := strings.Cut(name, "-")
	switch family {
	case m2Deletion:
		if err := s.gitSteps(ctx, repo, []string{"rm", "--", pathNote}); err != nil {
			return err
		}
	case m2Rename:
		if err := s.gitSteps(ctx, repo, []string{"mv", "--", m2PathOld, m2PathNew}); err != nil {
			return err
		}
	case categoryPolicy:
		return s.m2PolicyState(ctx, repo, name)
	}
	if err := materialize(repo, files.index); err != nil {
		return err
	}
	if err := s.gitSteps(ctx, repo, []string{gitAdd, "."}); err != nil {
		return err
	}
	return materialize(repo, files.checkout)
}

func (s *suite) m2PolicyState(ctx context.Context, repo, name string) error {
	path := filepath.Join(repo, policyFile)
	body, err := readFile(path)
	if err != nil {
		return err
	}
	file, err := policy.Decode(body)
	if err != nil {
		return fmt.Errorf("decode staged fixture policy: %w", err)
	}
	baseline := fmt.Sprintf("file_lines: %d", m2FileLineLimit)
	if file.Checks.Sizes == nil || file.Checks.Sizes.FileLines == nil ||
		*file.Checks.Sizes.FileLines != m2FileLineLimit ||
		strings.Count(string(body), baseline) != 1 {
		return fmt.Errorf("generated policy lacks the baseline file_lines: %w", ErrAcceptance)
	}
	limit := "file_lines: 301"
	if name == "policy-near" {
		limit = "file_lines: 299"
	}
	changed := strings.Replace(string(body), baseline, limit, 1)
	if err := writeFile(path, []byte(changed), fileMode); err != nil {
		return err
	}
	if err := s.gitSteps(ctx, repo, []string{gitAdd, "--", policyFile}); err != nil {
		return err
	}
	return writeFile(path, body, fileMode)
}

func (s *suite) m2AlternateRepository(ctx context.Context, repo, name string) (string, error) {
	linked := repo + "-linked"
	if err := s.gitSteps(ctx, repo, []string{"worktree", "add", "-q", "-b", "staged-alternate", linked}); err != nil {
		return "", err
	}
	if err := s.m2LinkedEnvironment(repo, linked); err != nil {
		return "", err
	}
	if err := s.m2Initialize(ctx, linked); err != nil {
		return "", err
	}
	if name == "alternate-near" {
		if err := materialize(linked, map[string]string{pathNote: m2RedText()}); err != nil {
			return "", err
		}
		if err := s.gitSteps(ctx, linked, []string{gitAdd, "--", pathNote}); err != nil {
			return "", err
		}
	}
	index := filepath.Join(environmentValue(s.env, "GIT_DIR"), m2AlternateIndex)
	s.env = append(s.env, "GIT_INDEX_FILE="+index)
	if err := s.gitSteps(ctx, linked, []string{"read-tree", "HEAD"}); err != nil {
		return "", err
	}
	return linked, nil
}

func (s *suite) m2LinkedEnvironment(repo, linked string) error {
	body, err := readFile(filepath.Join(linked, gitDirectory))
	if err != nil {
		return err
	}
	dir, found := strings.CutPrefix(strings.TrimSpace(string(body)), "gitdir: ")
	if !found {
		return fmt.Errorf("linked fixture lacks its Git directory: %w", ErrAcceptance)
	}
	root, err := filepath.EvalSymlinks(linked)
	if err != nil {
		return fmt.Errorf("resolve linked fixture root: %w", err)
	}
	dir, err = filepath.Rel(root, dir)
	if err != nil {
		return fmt.Errorf("relative linked Git directory: %w", err)
	}
	common := filepath.Join(repo, gitDirectory)
	s.env = append(s.env, "GIT_DIR="+dir, "GIT_COMMON_DIR="+common, "GIT_PREFIX=")
	return nil
}
