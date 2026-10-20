package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/setup"
)

const m2SetupCommand = "setup"

func (s *suite) setupM2(ctx context.Context, repo string, env []string) ([]string, error) {
	if !s.m2.Ready {
		return nil, fmt.Errorf("shared M2 tools are not verified: %w", ErrAcceptance)
	}
	env, err := s.m2PrerequisiteEnvironment(env, s.m2.Bin)
	if err != nil {
		return nil, err
	}
	if err := s.m2VerifyTools(ctx, repo, env, s.m2.Bin); err != nil {
		return nil, err
	}
	if err := s.m2RunSetup(ctx, repo, env); err != nil {
		return nil, err
	}
	if err := s.m2VerifySetup(ctx, repo, env); err != nil {
		return nil, err
	}
	return env, nil
}

func (s *suite) m2RunSetup(ctx context.Context, repo string, env []string, args ...string) error {
	return success(s.execute(ctx, command{
		program: s.binary, args: append([]string{m2SetupCommand}, args...), dir: repo, env: env,
	}))
}

func (s *suite) m2SetupClean(ctx context.Context, repo string) error {
	s.m2.Ready = false
	if err := os.RemoveAll(s.m2.Bin); err != nil {
		return fmt.Errorf("clear M2 tool directory: %w", err)
	}
	if err := os.MkdirAll(s.m2.Bin, dirMode); err != nil {
		return fmt.Errorf("create M2 tool directory: %w", err)
	}
	env, err := s.m2PrerequisiteEnvironment(s.env, s.m2.Bin)
	if err != nil {
		return err
	}
	if err := m2MissingTools(env); err != nil {
		return err
	}
	if err := s.m2RunSetup(ctx, repo, env); err != nil {
		return err
	}
	if err := s.m2VerifySetup(ctx, repo, env); err != nil {
		return err
	}
	s.m2.Ready, s.env = true, env
	return nil
}

func (s *suite) m2SetupIdempotent(ctx context.Context, repo string) error {
	env, err := s.setupM2(ctx, repo, s.env)
	if err != nil {
		return err
	}
	return s.m2RepeatSetup(ctx, repo, env)
}

func (s *suite) m2RepeatSetup(ctx context.Context, repo string, env []string) error {
	before, err := s.m2SetupHashes(ctx, repo, env)
	if err != nil {
		return err
	}
	got := s.execute(ctx, command{program: s.binary, args: []string{m2SetupCommand}, dir: repo, env: env})
	if runErr := success(got); runErr != nil {
		return runErr
	}
	if got.stdout != "" || got.stderr != "" {
		return fmt.Errorf("repeated M2 setup was not silent: %w", ErrAcceptance)
	}
	after, err := s.m2SetupHashes(ctx, repo, env)
	if err != nil {
		return err
	}
	if !maps.Equal(before, after) {
		return fmt.Errorf("repeated M2 setup changed owned state: %w", ErrAcceptance)
	}
	return s.m2VerifySetup(ctx, repo, env)
}

func (s *suite) m2SetupUninstall(ctx context.Context, repo string) error {
	env, err := s.setupM2(ctx, repo, s.env)
	if err != nil {
		return err
	}
	path, err := s.m2ReceiptAt(repo, env)
	if err != nil {
		return err
	}
	receipt, err := setup.ReadReceipt(path)
	if err != nil {
		return fmt.Errorf("read M2 uninstall receipt: %w", err)
	}
	local := filepath.Join(receipt.HooksPath, "post-merge")
	if writeErr := writeFile(local, []byte(m2LocalEdit), execMode); writeErr != nil {
		return writeErr
	}
	before, err := treeDigests(s.m2.Bin)
	if err != nil {
		return err
	}
	if err := s.m2RunSetup(ctx, repo, env, "--uninstall"); err != nil {
		return err
	}
	return s.m2VerifyUninstall(path, local, receipt, before)
}

func (s *suite) m2VerifyUninstall(path, local string, receipt setup.Receipt, before map[string]string) error {
	paths := make([]string, 0, len(receipt.Hooks)+1)
	paths = append(paths, path)
	for _, hook := range receipt.Hooks {
		paths = append(paths, hook.Path)
	}
	for _, owned := range paths {
		if _, err := os.Lstat(owned); !os.IsNotExist(err) {
			return fmt.Errorf("M2 uninstall retained owned path %s: %w", owned, ErrAcceptance)
		}
	}
	body, err := readFile(local)
	if err != nil {
		return err
	}
	if string(body) != m2LocalEdit {
		return fmt.Errorf("M2 uninstall changed an unrelated hook: %w", ErrAcceptance)
	}
	after, err := treeDigests(s.m2.Bin)
	if err != nil {
		return err
	}
	if !maps.Equal(before, after) {
		return fmt.Errorf("M2 uninstall changed installed tools: %w", ErrAcceptance)
	}
	return nil
}
