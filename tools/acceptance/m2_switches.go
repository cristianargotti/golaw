package main

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
)

func (s *suite) m2SwitchCase(ctx context.Context, repo, name string) error {
	rungName, suffix, err := m2RungCase(name)
	if err != nil {
		return err
	}
	if suffix != "disabled" {
		return fmt.Errorf("unknown M2 switch case: %w", ErrAcceptance)
	}
	if armErr := s.m2ArmPin(ctx, repo, m2OlderVersion); armErr != nil {
		return armErr
	}
	if switchErr := m2DisableRung(repo, rungName); switchErr != nil {
		return switchErr
	}
	return s.m2SilentRung(ctx, repo, rungName)
}

func m2DisableRung(repo, rungName string) error {
	path := filepath.Join(repo, policyFile)
	body, err := readFile(path)
	if err != nil {
		return err
	}
	if _, decodeErr := policy.Decode(body); decodeErr != nil {
		return fmt.Errorf("decode M2 switch policy: %w", decodeErr)
	}
	key := strings.ReplaceAll(rungName, "-", "_")
	before, after := key+": true", key+": false"
	if strings.Count(string(body), before) != 1 {
		return fmt.Errorf("M2 switch fixture lacks enabled %s: %w", key, ErrAcceptance)
	}
	return writeFile(path, []byte(strings.Replace(string(body), before, after, 1)), fileMode)
}

func (s *suite) m2SilentRung(ctx context.Context, repo, rungName string) error {
	before, err := s.captureM2(ctx, repo, s.env)
	if err != nil {
		return err
	}
	state, err := s.m2StateDir(s.env)
	if err != nil {
		return err
	}
	enrollment, err := treeDigests(state)
	if err != nil {
		return err
	}
	env, err := s.m2PrerequisiteEnvironment(s.env, "")
	if err != nil {
		return err
	}
	if err := m2MissingTools(env); err != nil {
		return err
	}
	got := s.m2InvokeRung(ctx, repo, env, rungName, true)
	if runErr := success(got); runErr != nil {
		return runErr
	}
	if got.stdout != "" || got.stderr != "" {
		return fmt.Errorf("disabled rung was not silent: %w", ErrAcceptance)
	}
	return s.m2UnchangedRung(ctx, repo, state, before, enrollment)
}

func (s *suite) m2UnchangedRung(ctx context.Context, repo, state string, before m2State, enrollment map[string]string) error {
	after, err := s.captureM2(ctx, repo, s.env)
	if err != nil {
		return err
	}
	record, err := treeDigests(state)
	if err != nil {
		return err
	}
	if before.Head != after.Head || before.Tree != after.Tree || !maps.Equal(before.Files, after.Files) ||
		!maps.Equal(before.Indexes, after.Indexes) || !maps.Equal(enrollment, record) {
		return fmt.Errorf("disabled rung changed repository or enrollment: %w", ErrAcceptance)
	}
	return nil
}
