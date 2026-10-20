package main

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/setup"
)

func (s *suite) installedM2(ctx context.Context, repo, rungName, input string, args, env []string) (command, error) {
	hook, err := s.m2InstalledPath(ctx, repo, rungName, env)
	if err != nil {
		return command{}, err
	}
	shell, err := s.lookup("sh")
	if err != nil {
		return command{}, fmt.Errorf("locate installed hook shell: %w", err)
	}
	if err := m2RequireExecutable(shell); err != nil {
		return command{}, err
	}
	if s.m2OS == goosWindows && environmentValue(env, "GOLAW_M2_RECORDERS") != "" {
		if err := s.m2WindowsProbe(ctx, shell, repo, env); err != nil {
			return command{}, err
		}
	}
	return command{
		program: shell, args: append([]string{hook}, args...),
		dir: repo, env: slices.Clone(env), input: input,
	}, nil
}

func (s *suite) m2InstalledPath(ctx context.Context, repo, rungName string, env []string) (string, error) {
	if !slices.Contains(rung.Names(), rungName) {
		return "", fmt.Errorf("unknown installed rung %q: %w", rungName, ErrAcceptance)
	}
	path, err := s.m2ReceiptAt(repo, env)
	if err != nil {
		return "", err
	}
	receipt, err := setup.ReadReceipt(path)
	if err != nil {
		return "", fmt.Errorf("read installed rung receipt: %w", err)
	}
	if identityErr := s.m2ReceiptIdentity(repo, receipt); identityErr != nil {
		return "", identityErr
	}
	hooks, err := s.m2EffectiveHooks(ctx, repo, env)
	if err != nil {
		return "", err
	}
	if hooks != receipt.HooksPath {
		return "", fmt.Errorf("installed rung routing differs from receipt: %w", ErrAcceptance)
	}
	if err := m2ReceiptArtifacts(receipt, s.m2.Bin); err != nil {
		return "", err
	}
	return filepath.Join(hooks, rungName), nil
}

func (s *suite) m2RunInstalled(ctx context.Context, repo, rungName, input string, args, env []string) result {
	invocation, err := s.installedM2(ctx, repo, rungName, input, args, env)
	if err != nil {
		return result{err: err}
	}
	return s.execute(ctx, invocation)
}

func (s *suite) m2StableInstalled(ctx context.Context, repo string, red bool, evidence string) error {
	before, err := s.captureM2Installed(ctx, repo, s.env)
	if err != nil {
		return err
	}
	got := s.m2RunInstalled(ctx, repo, rung.PreCommit, "", nil, s.env)
	after, err := s.captureM2Installed(ctx, repo, s.env)
	if err != nil {
		return err
	}
	if before.Head != after.Head || before.Tree != after.Tree ||
		!maps.Equal(before.Files, after.Files) || !maps.Equal(before.Indexes, after.Indexes) {
		return fmt.Errorf("installed pre-commit changed fixture state: %w", ErrAcceptance)
	}
	return m2JudgeNative(got, red, evidence, true)
}
