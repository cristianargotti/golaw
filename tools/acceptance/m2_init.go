package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/setup"
)

type m2RoutingState struct {
	dir   string
	files map[string]string
}

func (s *suite) m2InitUnarmed(ctx context.Context, repo string) error {
	before, err := s.m2UnarmedState(ctx, repo)
	if err != nil {
		return err
	}
	if initErr := s.m2Initialize(ctx, repo); initErr != nil {
		return initErr
	}
	after, err := s.m2UnarmedState(ctx, repo)
	if err != nil {
		return err
	}
	if before.dir != after.dir || !maps.Equal(before.files, after.files) {
		return fmt.Errorf("init changed effective hook routing or files: %w", ErrAcceptance)
	}
	return s.m2CheckEnrollment(repo, s.env)
}

func (s *suite) m2UnarmedState(ctx context.Context, repo string) (m2RoutingState, error) {
	if err := s.m2SetupAbsent(repo, s.env); err != nil {
		return m2RoutingState{}, err
	}
	dir, err := s.m2EffectiveHooks(ctx, repo, s.env)
	if err != nil {
		return m2RoutingState{}, err
	}
	files, err := m2OptionalDigests(dir)
	return m2RoutingState{dir: dir, files: files}, err
}

func (s *suite) m2SetupAbsent(repo string, env []string) error {
	path, err := s.m2ReceiptAt(repo, env)
	if err != nil {
		return err
	}
	_, err = setup.ReadReceipt(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("repository unexpectedly has setup state: %w", errors.Join(ErrAcceptance, err))
}

func (s *suite) m2InitIdempotent(ctx context.Context, repo string) error {
	if err := s.m2Initialize(ctx, repo); err != nil {
		return err
	}
	before, err := s.m2InitDigests(repo)
	if err != nil {
		return err
	}
	if initErr := m2InitNoDiff(s.m2LifecycleCommand(ctx, repo, commandInit, flagApply)); initErr != nil {
		return initErr
	}
	after, err := s.m2InitDigests(repo)
	if err != nil {
		return err
	}
	if !maps.Equal(before, after) {
		return fmt.Errorf("second init changed generated files or enrollment: %w", ErrAcceptance)
	}
	return s.m2CheckEnrollment(repo, s.env)
}

func (s *suite) m2InitDigests(repo string) (map[string]string, error) {
	files, err := m2CheckoutDigests(repo, filepath.Join(repo, gitDirectory))
	if err != nil {
		return nil, err
	}
	state, err := s.m2StateDir(s.env)
	if err != nil {
		return nil, err
	}
	records, err := treeDigests(state)
	if err != nil {
		return nil, err
	}
	maps.Copy(files, records)
	return files, nil
}

func m2InitNoDiff(got result) error {
	if err := success(got); err != nil {
		return err
	}
	if got.stderr != "" {
		return fmt.Errorf("second init reported an error: %w", ErrAcceptance)
	}
	for _, marker := range []string{
		"+++ ", "--- ", "Gravações:", "Propostas sem gravação:", "Writes:", "Proposals without writes:",
	} {
		if strings.Contains(got.stdout, marker) {
			return fmt.Errorf("second init reported a diff or write: %w", ErrAcceptance)
		}
	}
	return nil
}

func (s *suite) m2CheckEnrollment(repo string, env []string) error {
	data, err := readFile(filepath.Join(repo, policyFile))
	if err != nil {
		return err
	}
	file, err := policy.Decode(data)
	if err != nil {
		return fmt.Errorf("decode lifecycle policy: %w", err)
	}
	root, _, err := install.ProjectIdentity(repo)
	if err != nil {
		return fmt.Errorf("identify lifecycle enrollment: %w", err)
	}
	state, err := s.m2StateDir(env)
	if err != nil {
		return err
	}
	record, found, err := policy.ReadRecord(state, root)
	if err != nil {
		return fmt.Errorf("read lifecycle enrollment: %w", err)
	}
	if !found || record.Root != root || record.KitVersion != m2CandidateVersion ||
		file.Kit.Version != m2CandidateVersion || record.PolicyDigest != policy.Digest(file) {
		return fmt.Errorf("lifecycle enrollment or kit pin does not match: %w", ErrAcceptance)
	}
	return nil
}

func m2OptionalDigests(dir string) (map[string]string, error) {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	return treeDigests(dir)
}
