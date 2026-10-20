package main

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
)

func (s *suite) m2DispatchUnarmed(ctx context.Context, repo string) error {
	if !s.m2.Ready {
		return fmt.Errorf("dispatch requires verified tools: %w", ErrAcceptance)
	}
	env, err := s.m2PrerequisiteEnvironment(s.env, s.m2.Bin)
	if err != nil {
		return err
	}
	s.env = env
	if err := s.m2VerifyTools(ctx, repo, env, s.m2.Bin); err != nil {
		return err
	}
	if err := s.gitSteps(ctx, repo, []string{gitAdd, "."}, commitArgs("test: prepare adoption fixture")); err != nil {
		return err
	}
	if err := s.m2WriteDispatchers(ctx, repo); err != nil {
		return err
	}
	if err := s.m2Initialize(ctx, repo); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(repo, pathNote), []byte(m2RedText()), fileMode); err != nil {
		return err
	}
	if err := s.m2CommitThrough(ctx, repo, "feat: adopt the golaw policy"); err != nil {
		return err
	}
	return s.m2SetupAbsent(repo, s.env)
}

func (s *suite) m2WriteDispatchers(ctx context.Context, repo string) error {
	dir, err := s.m2EffectiveHooks(ctx, repo, s.env)
	if err != nil {
		return err
	}
	for _, rung := range []string{"pre-commit", "commit-msg", "pre-push"} {
		body := "#!/bin/sh\nexec lefthook run " + rung + " \"$@\"\n"
		if err := writeFile(filepath.Join(dir, rung), []byte(body), execMode); err != nil {
			return err
		}
	}
	return nil
}

func (s *suite) m2CommitThrough(ctx context.Context, repo, message string) error {
	before, err := s.m2ObserveGit(ctx, repo, s.env, m2RevParse, m2Verify, m2Head)
	if err != nil {
		return err
	}
	if stageErr := s.gitSteps(ctx, repo, []string{gitAdd, "."}); stageErr != nil {
		return stageErr
	}
	got := s.execute(ctx, command{program: s.git, args: commitArgs(message), dir: repo, env: s.env})
	if commitErr := success(got); commitErr != nil {
		return commitErr
	}
	if strings.Contains(got.stdout+got.stderr, "text.dashes") {
		return fmt.Errorf("unarmed or upgrade commit emitted a finding: %w", ErrAcceptance)
	}
	after, err := s.m2ObserveGit(ctx, repo, s.env, m2RevParse, m2Verify, m2Head)
	if err != nil {
		return err
	}
	if before == after {
		return fmt.Errorf("lifecycle commit did not advance HEAD: %w", ErrAcceptance)
	}
	return s.m2CommittedPin(ctx, repo)
}

func (s *suite) m2CommittedPin(ctx context.Context, repo string) error {
	body, err := s.m2ObserveGit(ctx, repo, s.env, "show", "HEAD:"+policyFile)
	if err != nil {
		return err
	}
	file, err := policy.Decode([]byte(body))
	if err != nil {
		return fmt.Errorf("decode committed lifecycle policy: %w", err)
	}
	if file.Kit.Version != m2CandidateVersion {
		return fmt.Errorf("lifecycle commit has the wrong kit pin: %w", ErrAcceptance)
	}
	return nil
}

func (s *suite) m2UpgradeCommit(ctx context.Context, repo string) error {
	env, err := s.setupM2(ctx, repo, s.env)
	if err != nil {
		return err
	}
	s.env = env
	if err := s.m2CommitPin(ctx, repo, m2OlderVersion); err != nil {
		return err
	}
	if err := success(s.m2LifecycleCommand(ctx, repo, commandInit, m2UpdateFlag, flagApply)); err != nil {
		return err
	}
	if err := s.m2CheckEnrollment(repo, s.env); err != nil {
		return err
	}
	return s.m2CommitThrough(ctx, repo, "feat: update the golaw policy")
}

func (s *suite) m2UpdatePreservesEdit(ctx context.Context, repo string) error {
	if err := s.m2CommitPin(ctx, repo, m2OlderVersion); err != nil {
		return err
	}
	path := filepath.Join(repo, m2EditedAsset)
	body, err := readFile(path)
	if err != nil {
		return err
	}
	body = append(body, []byte(m2LocalEdit)...)
	if writeErr := writeFile(path, body, fileMode); writeErr != nil {
		return writeErr
	}
	if previewErr := s.m2UpdatePreview(ctx, repo); previewErr != nil {
		return previewErr
	}
	got := s.m2LifecycleCommand(ctx, repo, commandInit, m2UpdateFlag, flagApply)
	if applyErr := answered(got, 0, m2EditedAsset); applyErr != nil {
		return applyErr
	}
	after, err := readFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(body, after) {
		return fmt.Errorf("update replaced the local edit: %w", ErrAcceptance)
	}
	return s.m2CheckEnrollment(repo, s.env)
}

func (s *suite) m2UpdatePreview(ctx context.Context, repo string) error {
	before, err := s.m2InitDigests(repo)
	if err != nil {
		return err
	}
	if previewErr := answered(s.m2LifecycleCommand(ctx, repo, commandInit, m2UpdateFlag), 0, m2EditedAsset); previewErr != nil {
		return previewErr
	}
	after, err := s.m2InitDigests(repo)
	if err != nil {
		return err
	}
	if !maps.Equal(before, after) {
		return fmt.Errorf("update preview changed files or enrollment: %w", ErrAcceptance)
	}
	return nil
}
