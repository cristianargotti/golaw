package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/rung"
)

// The fresh machine rows and the human line of a missing bundle (D8).
const (
	m5FreshCategory = "m5-fresh"
	m5AgentCommand  = "agent"
	m5HumanMissing  = "Perfil " + m5Name + " ausente ou divergente: execute " + m5InstallCommand
)

// m5Fresh is the repository of the first machine and its clone on a
// second machine that never installed the bundle.
type m5Fresh struct {
	setup  m5Setup
	clone  *suite
	cloned string
}

// m5FreshCases are the governed operations of D8 in the order of the
// contract's rows, each with the case that runs it.
func m5FreshCases() []m5Case {
	names := []string{"scan", "scan-enrolled", "rung", "hook-enrolled", "init-apply", "agent-install", "unenrolled-allow"}
	runs := []func(*suite, context.Context) error{
		(*suite).m5FreshScan, (*suite).m5FreshEnrolled, (*suite).m5FreshRung, (*suite).m5FreshHook,
		(*suite).m5FreshInit, (*suite).m5FreshAgent, (*suite).m5FreshAllow,
	}
	cases := make([]m5Case, len(names))
	for i := range names {
		cases[i] = m5Case{name: names[i], run: runs[i]}
	}
	return cases
}

// m5Machines adopts a repository on the case's machine and clones it on a
// home of its own, the fresh clone of A19.
func (s *suite) m5Machines(ctx context.Context, name string) (m5Fresh, error) {
	setup, err := s.m5CleanRepository(ctx, name)
	if err != nil {
		return m5Fresh{}, err
	}
	second := s.m5Scope("second", name)
	cloned := filepath.Join(second.base, dirRepos, name)
	if stepErr := os.MkdirAll(filepath.Dir(cloned), dirMode); stepErr != nil {
		return m5Fresh{}, fmt.Errorf("create the second machine: %w", stepErr)
	}
	if stepErr := second.gitSteps(ctx, filepath.Dir(cloned), []string{"clone", m2Quiet, setup.repo, cloned}); stepErr != nil {
		return m5Fresh{}, stepErr
	}
	return m5Fresh{setup: setup, clone: second, cloned: cloned}, nil
}

// m5Enrolled is the value scan --enrolled reads: the policy digest and the
// pinned profile digest of a repository.
func m5Enrolled(repo string) (string, error) {
	file, err := m5DecodePolicy(repo)
	if err != nil {
		return "", err
	}
	if file.Profile == nil {
		return "", fmt.Errorf("the policy of %s pins no profile: %w", repo, ErrAcceptance)
	}
	return policy.Digest(file) + "," + file.Profile.Digest, nil
}

// m5Hook sends a write of note.md to the agent hook in a repository.
func (s *suite) m5Hook(ctx context.Context, repo string) (result, error) {
	body, err := payload(s.root, repo, fixture{
		name: "m5-fresh", event: eventPre, tool: toolWrite,
		input: map[string]any{"file_path": filepath.Join(repo, "note.md"), "content": "ok\n"},
	})
	if err != nil {
		return result{}, err
	}
	return s.execute(ctx, command{program: s.binary, args: []string{commandHook, agentClaude}, dir: repo, env: s.env, input: string(body)}), nil
}

// m5JudgeDenial: the hook denies with the command for the agent and the
// pt-BR line for the person.
func m5JudgeDenial(got result) error {
	if err := expect(got, wantDeny); err != nil {
		return err
	}
	var decoded answer
	if err := json.Unmarshal([]byte(got.stdout), &decoded); err != nil {
		return fmt.Errorf("decode the denial: %w", err)
	}
	if !strings.Contains(decoded.Hook.Reason, m5InstallCommand) || !strings.Contains(decoded.Human, m5HumanMissing) {
		return fmt.Errorf("the denial reads %q and %q: %w", decoded.Hook.Reason, decoded.Human, ErrAcceptance)
	}
	return nil
}

// m5Untouched: the refusal wrote nothing in the clone and enrolled nothing
// on its machine.
func (s *suite) m5Untouched(ctx context.Context, repo string) error {
	status := s.execute(ctx, command{program: s.git, args: []string{gitStatus, gitPorcelain}, dir: repo, env: s.env})
	if err := success(status); err != nil {
		return err
	}
	if status.stdout != "" {
		return fmt.Errorf("the refusal changed the clone: %w", ErrAcceptance)
	}
	dirs, err := s.m5Dirs()
	if err != nil {
		return err
	}
	_, found, err := policy.FindRecord(dirs.State, repo)
	if err != nil {
		return fmt.Errorf("read the enrollment of the clone: %w", err)
	}
	if found {
		return fmt.Errorf("the refusal enrolled the clone: %w", ErrAcceptance)
	}
	return nil
}

func (s *suite) m5FreshScan(ctx context.Context) error {
	f, err := s.m5Machines(ctx, "scan")
	if err != nil {
		return err
	}
	return m5ScanRefusal(f.clone.m5Golaw(ctx, f.cloned, commandScan, "."), m5InstallCommand)
}

func (s *suite) m5FreshEnrolled(ctx context.Context) error {
	f, err := s.m5Machines(ctx, "scan-enrolled")
	if err != nil {
		return err
	}
	digests, err := m5Enrolled(f.cloned)
	if err != nil {
		return err
	}
	return m5ScanRefusal(f.clone.m5Golaw(ctx, f.cloned, commandScan, "--enrolled", digests, "."), m5InstallCommand)
}

// m5FreshRung arms the rung while the bundle is installed, as setup needs,
// then takes the bundle away.
func (s *suite) m5FreshRung(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "rung")
	if err != nil {
		return err
	}
	env, err := s.m5Arm(ctx, setup.repo)
	if err != nil {
		return err
	}
	if stepErr := success(s.m5Profile(ctx, m5RemoveSub, m5Name)); stepErr != nil {
		return stepErr
	}
	got := s.execute(ctx, command{program: s.binary, args: []string{commandHook, rung.PreCommit}, dir: setup.repo, env: env})
	return m5OneLine(got)
}

func (s *suite) m5FreshHook(ctx context.Context) error {
	setup, err := s.m5CleanRepository(ctx, "hook")
	if err != nil {
		return err
	}
	if stepErr := success(s.m5Profile(ctx, m5RemoveSub, m5Name)); stepErr != nil {
		return stepErr
	}
	got, err := s.m5Hook(ctx, setup.repo)
	if err != nil {
		return err
	}
	return m5JudgeDenial(got)
}

func (s *suite) m5FreshInit(ctx context.Context) error {
	f, err := s.m5Machines(ctx, "init")
	if err != nil {
		return err
	}
	if stepErr := m5OneLine(f.clone.m5Golaw(ctx, f.cloned, commandInit, flagApply)); stepErr != nil {
		return stepErr
	}
	return f.clone.m5Untouched(ctx, f.cloned)
}

func (s *suite) m5FreshAgent(ctx context.Context) error {
	f, err := s.m5Machines(ctx, "agent")
	if err != nil {
		return err
	}
	if stepErr := m5OneLine(f.clone.m5Golaw(ctx, f.cloned, m5AgentCommand, m5InstallSub, "--agent", agentClaude)); stepErr != nil {
		return stepErr
	}
	return f.clone.m5Untouched(ctx, f.cloned)
}

func (s *suite) m5FreshAllow(ctx context.Context) error {
	f, err := s.m5Machines(ctx, "allow")
	if err != nil {
		return err
	}
	got, err := f.clone.m5Hook(ctx, f.cloned)
	if err != nil {
		return err
	}
	return expect(got, wantAllow)
}
