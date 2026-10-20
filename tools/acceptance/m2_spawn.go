package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
)

const (
	m2SpawnFile = "git_internal_test.go"
	m2SpawnRed  = `package probe

import "testing"

func TestStagedFailure(t *testing.T) {
	t.Parallel()
	t.Fatal("staged failure reached")
}
`
)

func (s *suite) m2SpawnGit(ctx context.Context, repo string, red bool) error {
	linked, err := s.m2SpawnRepository(ctx, repo)
	if err != nil {
		return err
	}
	if err := s.m2SpawnFiles(ctx, linked, red); err != nil {
		return err
	}
	if err := m2EnableStagedTests(linked); err != nil {
		return err
	}
	return s.m2SpawnInstalled(ctx, linked, red)
}

func (s *suite) m2SpawnRepository(ctx context.Context, repo string) (string, error) {
	originalPath := environmentValue(s.env, envPath)
	linked, err := s.m2AlternateRepository(ctx, repo, "spawn-git")
	if err != nil {
		return "", err
	}
	env := runner.UserEnv(s.env)
	for _, key := range runner.HookVariables() {
		value := environmentValue(s.env, key)
		if key == "GIT_WORK_TREE" {
			value = "."
		}
		env = append(env, key+"="+value)
	}
	env, err = s.setupM2(ctx, linked, env)
	if err != nil {
		return "", err
	}
	for i, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, envPath) && originalPath != "" {
			env[i] = entry + string(os.PathListSeparator) + originalPath
		}
	}
	s.env = env
	return linked, nil
}

func (s *suite) m2SpawnFiles(ctx context.Context, repo string, red bool) error {
	path := filepath.Join(s.root, dirTools, dirAcceptance, m2Testdata, "m2", "git_test.go.txt")
	passing, err := readFile(path)
	if err != nil {
		return err
	}
	staged, checkout := passing, []byte(m2SpawnRed)
	if red {
		staged, checkout = checkout, staged
	}
	path = filepath.Join(repo, m2SpawnFile)
	if err := writeFile(path, staged, fileMode); err != nil {
		return err
	}
	if err := s.gitSteps(ctx, repo, []string{gitAdd, "--", m2SpawnFile}); err != nil {
		return err
	}
	return writeFile(path, checkout, fileMode)
}

func m2EnableStagedTests(repo string) error {
	path := filepath.Join(repo, policyFile)
	body, err := readFile(path)
	if err != nil {
		return err
	}
	file, err := policy.Decode(body)
	if err != nil {
		return fmt.Errorf("decode child Git fixture policy: %w", err)
	}
	const baseline = "test_staged: false"
	if file.Ladder == nil || file.Ladder.TestStaged == nil || *file.Ladder.TestStaged ||
		strings.Count(string(body), baseline) != 1 {
		return fmt.Errorf("child Git fixture lacks disabled staged tests: %w", ErrAcceptance)
	}
	changed := strings.Replace(string(body), baseline, "test_staged: true", 1)
	return writeFile(path, []byte(changed), fileMode)
}

func (s *suite) m2SpawnInstalled(ctx context.Context, repo string, red bool) error {
	return s.m2StableInstalled(ctx, repo, red, "staged failure reached")
}
