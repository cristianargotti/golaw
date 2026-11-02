package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// The budgets of M7.13 row 3 at the 95th percentile of 20 warm runs: a
// port edit, whose regeneration runs in process, and an edit a pinned
// generator turns into code.
const (
	m7PortBudget  = time.Second
	m7ToolBudget  = 3 * time.Second
	m7SlowTool    = "sqlc"
	m7LacksNotice = "the agent context %q lacks the notice %q: %w"
)

// m7P95 samples c and answers its 95th percentile.
func (s *m7Suite) m7P95(ctx context.Context, c command) (time.Duration, error) {
	samples, err := s.m6Sample(ctx, c)
	if err != nil {
		return 0, err
	}
	return m6P95(samples)
}

// m7PortP95: the port edit through Claude's Write with the receipts setup
// --gen wrote, so the golangci-lint fmt stage runs over every file the
// event wrote as an adopter configures it (M7.9.3).
func (s *m7Suite) m7PortP95(ctx context.Context) error {
	repo, _, after, err := s.m7Edited(ctx, "port-p95")
	if err != nil {
		return err
	}
	c, err := s.m6ClaudeCommand(repo, eventPost, m7PortsPath, after, s.m7GenEnv())
	if err != nil {
		return err
	}
	p95, err := s.m7P95(ctx, c)
	if err != nil || p95 <= m7PortBudget {
		return err
	}
	c.env = s.env
	bare, err := s.m7P95(ctx, c)
	if err == nil && bare <= m7PortBudget {
		return fmt.Errorf("a port edit regenerates at %s at the 95th percentile with the golangci-lint fmt stage and at %s without it, the budget is %s: "+
			"format only the files the event wrote and leave the appended stubs to go/format (A45): %w", p95, bare, m7PortBudget, ErrAcceptance)
	}
	return fmt.Errorf("a port edit regenerates at %s at the 95th percentile, the budget is %s: %w", p95, m7PortBudget, ErrAcceptance)
}

// m7ToolP95: an edit of path, which a pinned generator renders, through
// Claude's Write with the generators setup --gen built.
func (s *m7Suite) m7ToolP95(ctx context.Context, name, path string, edit func([]byte) ([]byte, error)) error {
	repo, _, after, err := s.m7EditContract(ctx, name, path, edit)
	if err != nil {
		return err
	}
	c, err := s.m6ClaudeCommand(repo, eventPost, path, after, s.m7GenEnv())
	if err != nil {
		return err
	}
	p95, err := s.m7P95(ctx, c)
	if err != nil {
		return err
	}
	if p95 > m7ToolBudget {
		return fmt.Errorf("an edit of %s regenerates at %s at the 95th percentile, the budget is %s: %w", path, p95, m7ToolBudget, ErrAcceptance)
	}
	return nil
}

// m7EditContract is a denial repository with path edited in the checkout,
// as the host writes it before its post event; it answers the bytes before
// and after the edit.
func (s *m7Suite) m7EditContract(ctx context.Context, name, path string, edit func([]byte) ([]byte, error)) (string, []byte, []byte, error) {
	repo, err := s.m7Denied(ctx, name)
	if err != nil {
		return "", nil, nil, err
	}
	full := filepath.Join(repo, filepath.FromSlash(path))
	before, err := readFile(full)
	if err != nil {
		return "", nil, nil, err
	}
	after, err := edit(before)
	if err != nil {
		return "", nil, nil, err
	}
	return repo, before, after, writeFile(full, after, fileMode)
}

// m7SlowGenerator: a query edit whose sqlc never answers is killed at the
// tool budget and allowed with the notice that names golaw gen.
func (s *m7Suite) m7SlowGenerator(ctx context.Context) error {
	env := m6WithData(s.env, filepath.Join(s.base, "slow-data"))
	toolsDir, err := policy.ToolsDir(runtime.GOOS, func(key string) string { return environmentValue(env, key) })
	if err != nil {
		return fmt.Errorf("resolve the slow tools directory: %w", err)
	}
	generators := runner.Generators()
	at := slices.IndexFunc(generators, func(t runner.Tool) bool { return t.Name == m7SlowTool })
	if at < 0 {
		return fmt.Errorf("the pinned generators list no %s (the generator row of C-7): %w", m7SlowTool, ErrAcceptance)
	}
	if buildErr := s.m6Sleeper(ctx, toolsDir, toolchain.GoTool(generators[at])); buildErr != nil {
		return buildErr
	}
	repo, _, after, err := s.m7EditContract(ctx, "slow-generator-killed", m7QueriesFile, func(b []byte) ([]byte, error) { return m7QueriesWithPurge(b), nil })
	if err != nil {
		return err
	}
	c, err := s.m6ClaudeCommand(repo, eventPost, m7QueriesFile, after, env)
	if err != nil {
		return err
	}
	return m7JudgeKilled(s.m6Timed(ctx, c))
}

// m7JudgeKilled wants the killed generator's write allowed inside the
// event budget with the notice that names golaw gen: never a denial, never
// a finding (M7.9.1).
func m7JudgeKilled(elapsed time.Duration, got result) error {
	decoded, err := m6KilledAnswer(elapsed, got)
	if err != nil {
		return err
	}
	if !strings.Contains(decoded.Hook.Additional, m7NotRegenerated) {
		return fmt.Errorf(m7LacksNotice, decoded.Hook.Additional, m7NotRegenerated, ErrAcceptance)
	}
	return nil
}
