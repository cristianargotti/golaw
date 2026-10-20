package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

func (s *suite) m2StateDir(env []string) (string, error) {
	dirs, err := policy.DefaultDirs(s.m2OS, func(name string) string { return environmentValue(env, name) })
	if err != nil {
		return "", fmt.Errorf("resolve M2 state directory: %w", err)
	}
	return dirs.State, nil
}

func (s *suite) m2ReceiptAt(repo string, env []string) (string, error) {
	_, identity, err := install.ProjectIdentity(repo)
	if err != nil {
		return "", fmt.Errorf("resolve M2 receipt identity: %w", err)
	}
	state, err := s.m2StateDir(env)
	if err != nil {
		return "", err
	}
	return setup.ReceiptPath(state, identity), nil
}

func (s *suite) m2EffectiveHooks(ctx context.Context, repo string, env []string) (string, error) {
	dir, err := s.m2ObserveGit(ctx, repo, env, m2RevParse, "--path-format=absolute", "--git-path", "hooks")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("M2 hooks directory is not absolute: %w", ErrAcceptance)
	}
	return dir, nil
}

func (s *suite) m2VerifySetup(ctx context.Context, repo string, env []string) error {
	path, err := s.m2ReceiptAt(repo, env)
	if err != nil {
		return err
	}
	receipt, err := setup.ReadReceipt(path)
	if err != nil {
		return fmt.Errorf("read M2 setup receipt: %w", err)
	}
	if identityErr := s.m2ReceiptIdentity(repo, receipt); identityErr != nil {
		return identityErr
	}
	hooks, err := s.m2EffectiveHooks(ctx, repo, env)
	if err != nil {
		return err
	}
	if hooks != receipt.HooksPath {
		return fmt.Errorf("M2 receipt hooks differ from effective routing: %w", ErrAcceptance)
	}
	if err := m2ReceiptArtifacts(receipt, environmentValue(env, m2EnvGoBin)); err != nil {
		return err
	}
	if err := s.m2VerifyTools(ctx, repo, env, environmentValue(env, m2EnvGoBin)); err != nil {
		return err
	}
	return s.m2VerifyDoctor(ctx, repo, env, path, receipt)
}

func (s *suite) m2ReceiptIdentity(repo string, receipt setup.Receipt) error {
	root, identity, err := install.ProjectIdentity(repo)
	if err != nil {
		return fmt.Errorf("resolve M2 setup identity: %w", err)
	}
	if receipt.Root != root || receipt.Identity != identity || receipt.Version != m2CandidateVersion {
		return fmt.Errorf("M2 receipt identity or version differs: %w", ErrAcceptance)
	}
	target, err := filepath.EvalSymlinks(receipt.Golaw)
	if err != nil {
		return fmt.Errorf("resolve M2 receipt candidate: %w", err)
	}
	binary, err := filepath.EvalSymlinks(s.binary)
	if err != nil {
		return fmt.Errorf("resolve M2 candidate: %w", err)
	}
	if target != binary {
		return fmt.Errorf("M2 receipt names another candidate: %w", ErrAcceptance)
	}
	return nil
}

func m2ReceiptArtifacts(receipt setup.Receipt, bin string) error {
	tools := runner.Toolchain()
	rungs := []string{"pre-commit", "commit-msg", "pre-push"}
	if len(receipt.Tools) != len(tools) || len(receipt.Hooks) != len(rungs) {
		return fmt.Errorf("M2 receipt lacks complete ownership: %w", ErrAcceptance)
	}
	for _, tool := range tools {
		want := setup.ToolRecord{Name: tool.Name, Version: tool.Version, Path: filepath.Join(bin, executable(tool.Name))}
		if !slices.Contains(receipt.Tools, want) {
			return fmt.Errorf("M2 receipt lacks pinned tool %s: %w", tool.Name, ErrAcceptance)
		}
	}
	for _, rung := range rungs {
		path := filepath.Join(receipt.HooksPath, rung)
		if err := m2RequireExecutable(path); err != nil {
			return err
		}
		body, err := readFile(path)
		if err != nil {
			return err
		}
		if !slices.Contains(receipt.Hooks, install.Artifact{Path: path, SHA256: install.Digest(body)}) {
			return fmt.Errorf("M2 receipt hook digest differs: %w", ErrAcceptance)
		}
	}
	return nil
}

func (s *suite) m2SetupHashes(ctx context.Context, repo string, env []string) (map[string]string, error) {
	path, err := s.m2ReceiptAt(repo, env)
	if err != nil {
		return nil, err
	}
	hooks, err := s.m2EffectiveHooks(ctx, repo, env)
	if err != nil {
		return nil, err
	}
	hashes, err := treeDigests(hooks)
	if err != nil {
		return nil, err
	}
	body, err := readFile(path)
	if err != nil {
		return nil, err
	}
	hashes[path] = install.Digest(body)
	return hashes, nil
}
