package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/rung"
)

const (
	m2OlderVersion = "0.1.0"
	m2NewerVersion = "0.3.0"
	m2MessageFile  = "message.txt"
	m2MessageBody  = "fix: exercise the rung\n"
)

func (s *suite) m2PinCase(ctx context.Context, repo, name string) error {
	rungName, suffix, err := m2RungCase(name)
	if err != nil {
		return err
	}
	versions := map[string]string{"older": m2OlderVersion, "equal": m2CandidateVersion, "newer": m2NewerVersion}
	version, found := versions[suffix]
	if !found {
		return fmt.Errorf("unknown M2 pin case: %w", ErrAcceptance)
	}
	if armErr := s.m2ArmPin(ctx, repo, version); armErr != nil {
		return armErr
	}
	if writeErr := writeFile(filepath.Join(repo, m2MessageFile), []byte(m2MessageBody), fileMode); writeErr != nil {
		return writeErr
	}
	got := s.m2InvokeRung(ctx, repo, s.env, rungName, false)
	if suffix == "equal" || got.err != nil {
		return success(got)
	}
	diagnostic := got.stdout + "\n" + got.stderr
	if got.code != 2 || !strings.Contains(diagnostic, version) ||
		!strings.Contains(diagnostic, m2CandidateVersion) ||
		!strings.Contains(diagnostic, "install the pinned version or run init --update") {
		return fmt.Errorf("rung pin refusal lacks both versions and repair: %w", ErrAcceptance)
	}
	return nil
}

func (s *suite) m2ArmPin(ctx context.Context, repo, version string) error {
	env, err := s.setupM2(ctx, repo, s.env)
	if err != nil {
		return err
	}
	s.env = env
	return s.m2CommitPin(ctx, repo, version)
}

func (s *suite) m2CommitPin(ctx context.Context, repo, version string) error {
	body, err := m2VersionPolicy(repo, version)
	if err != nil {
		return err
	}
	if writeErr := writeFile(filepath.Join(repo, policyFile), body, fileMode); writeErr != nil {
		return writeErr
	}
	if stageErr := s.gitSteps(ctx, repo, []string{gitAdd, "--", policyFile}); stageErr != nil {
		return stageErr
	}
	if commitErr := s.m2PlumbingCommit(ctx, repo); commitErr != nil {
		return commitErr
	}
	return s.m2EnrollPolicy(repo, body)
}

func m2VersionPolicy(repo, version string) ([]byte, error) {
	body, err := readFile(filepath.Join(repo, policyFile))
	if err != nil {
		return nil, err
	}
	file, err := policy.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("decode M2 pin policy: %w", err)
	}
	body, err = bootstrap.ManagedPolicy(body, version, file.Managed)
	if err != nil {
		return nil, fmt.Errorf("replace M2 policy pin: %w", err)
	}
	return body, nil
}

// Plumbing commits install the historical policy without invoking the
// newly armed hooks with a deliberately mismatched candidate.
func (s *suite) m2PlumbingCommit(ctx context.Context, repo string) error {
	tree, err := s.m2ObserveGit(ctx, repo, s.env, "write-tree")
	if err != nil {
		return err
	}
	head, err := s.m2ObserveGit(ctx, repo, s.env, m2RevParse, m2Verify, m2Head)
	if err != nil {
		return err
	}
	message := "test: pin acceptance policy"
	args := commitArgs(message)
	args = append(slices.Clone(args[:slices.Index(args, "commit")]),
		"commit-tree", tree, "-p", head, "-m", message)
	commit, err := s.m2ObserveGit(ctx, repo, s.env, args...)
	if err != nil {
		return err
	}
	return s.gitSteps(ctx, repo, []string{"update-ref", m2Head, commit, head})
}

func (s *suite) m2EnrollPolicy(repo string, body []byte) error {
	file, err := policy.Decode(body)
	if err != nil {
		return fmt.Errorf("decode M2 enrollment policy: %w", err)
	}
	root, _, err := install.ProjectIdentity(repo)
	if err != nil {
		return fmt.Errorf("identify M2 enrollment: %w", err)
	}
	state, err := s.m2StateDir(s.env)
	if err != nil {
		return err
	}
	record, found, err := policy.ReadRecord(state, root)
	if err != nil {
		return fmt.Errorf("read M2 pin enrollment: %w", err)
	}
	if !found {
		return fmt.Errorf("M2 pin fixture lacks enrollment: %w", ErrAcceptance)
	}
	record.KitVersion, record.PolicyDigest, record.Bootstrap = file.Kit.Version, policy.Digest(file), string(body)
	if writeErr := policy.WriteRecord(state, record); writeErr != nil {
		return fmt.Errorf("write M2 pin enrollment: %w", writeErr)
	}
	return nil
}

func m2RungCase(name string) (string, string, error) {
	cut := strings.LastIndex(name, "-")
	if cut < 0 || !slices.Contains(rung.Names(), name[:cut]) {
		return "", "", fmt.Errorf("unknown M2 rung case: %w", ErrAcceptance)
	}
	return name[:cut], name[cut+1:], nil
}

func (s *suite) m2InvokeRung(ctx context.Context, repo string, env []string, name string, disabled bool) result {
	args := []string{commandHook, name}
	input := ""
	switch name {
	case rung.CommitMsg:
		path := m2MessageFile
		if disabled {
			path = "missing-message.txt"
		}
		args = append(args, filepath.Join(repo, path))
	case rung.PrePush:
		args = append(args, "origin", "https://example.invalid/m2.git")
		if disabled {
			input = "malformed ref stream\n"
		}
	}
	return s.execute(ctx, command{program: s.binary, args: args, dir: repo, env: env, input: input})
}
