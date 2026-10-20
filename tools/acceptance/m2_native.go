package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runner"
)

const (
	m2WindowsOnly = "windows-shell-only"
	m2DashCheck   = "text.dashes"
	m2SamplePath  = "cmd/sample/main.go"
	m2SampleBody  = "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"sample\")\n}\n"
)

func (s *suite) nativeM2(ctx context.Context, name string) row {
	if name == m2WindowsOnly && s.m2OS != goosWindows {
		return row{"m2-native", name, statusSkip, "requires native Windows", false}
	}
	return measured("m2-native", name, s.m2NativeCase(ctx, name))
}

func m2NativeNames() []string {
	return []string{
		"installed-pre-commit-red", "installed-pre-commit-near",
		"installed-commit-msg-red", "installed-commit-msg-near",
		"installed-pre-push-red", "installed-pre-push-near",
		"spawn-git-red", "spawn-git-near", "windows-shell-only",
	}
}

func (s *suite) m2NativeCase(ctx context.Context, name string) error {
	if !slices.Contains(m2NativeNames(), name) {
		return fmt.Errorf("unknown M2 native case %q: %w", name, ErrAcceptance)
	}
	repo, err := s.m2NativeRepository(ctx, name)
	if err != nil {
		return err
	}
	red := strings.HasSuffix(name, "-red")
	if strings.HasPrefix(name, "spawn-git-") {
		return s.m2SpawnGit(ctx, repo, red)
	}
	env, err := s.setupM2(ctx, repo, s.env)
	if err != nil {
		return err
	}
	s.env = env
	if name == m2WindowsOnly {
		return s.m2WindowsRungs(ctx, repo)
	}
	rungName := strings.TrimPrefix(name[:strings.LastIndex(name, "-")], "installed-")
	switch rungName {
	case rung.PreCommit:
		return s.m2NativeStaged(ctx, repo, red)
	case rung.CommitMsg:
		return s.m2NativeMessage(ctx, repo, red, env)
	case rung.PrePush:
		env, err = s.m2PushEnvironment(ctx, repo, env, red)
		if err != nil {
			return err
		}
		return s.m2NativePush(ctx, repo, red, env)
	}
	return fmt.Errorf("unknown installed rung: %w", ErrAcceptance)
}

func (s *suite) m2NativeRepository(ctx context.Context, name string) (string, error) {
	repo, err := s.m2LifecycleRepository(ctx, name)
	if err != nil {
		return "", err
	}
	if err := materialize(repo, map[string]string{m2SamplePath: m2SampleBody, pathNote: m2Safe}); err != nil {
		return "", err
	}
	if err := s.m2LifecycleBaseline(ctx, repo); err != nil {
		return "", err
	}
	return repo, nil
}

func (s *suite) m2NativeStaged(ctx context.Context, repo string, red bool) error {
	indexed, checkout := m2RedText(), m2Changed
	if !red {
		indexed, checkout = checkout, indexed
	}
	path := filepath.Join(repo, pathNote)
	if err := writeFile(path, []byte(indexed), fileMode); err != nil {
		return err
	}
	if err := s.gitSteps(ctx, repo, []string{gitAdd, "--", pathNote}); err != nil {
		return err
	}
	if err := writeFile(path, []byte(checkout), fileMode); err != nil {
		return err
	}
	return s.m2StableInstalled(ctx, repo, red, m2DashCheck)
}

func (s *suite) m2NativeMessage(ctx context.Context, repo string, red bool, env []string) error {
	body := "fix: exercise installed routing\n"
	if red {
		body = "broken subject\n"
	}
	path := filepath.Join(s.base, m2MessageFile)
	if err := writeFile(path, []byte(body), fileMode); err != nil {
		return err
	}
	got := s.m2RunInstalled(ctx, repo, rung.CommitMsg, "", []string{path}, env)
	return m2JudgeNative(got, red, "commit.conventional", true)
}

func (s *suite) m2NativePush(ctx context.Context, repo string, red bool, env []string) error {
	head, err := s.m2ObserveGit(ctx, repo, env, m2RevParse, m2Verify, m2Head)
	if err != nil {
		return err
	}
	input := "refs/heads/main " + head + " refs/heads/main " + runner.ZeroSHA + "\n"
	args := []string{"local", filepath.Join(s.base, "local-remote")}
	got := s.m2RunInstalled(ctx, repo, rung.PrePush, input, args, env)
	return m2JudgeNative(got, red, "GO-2026-0001", true)
}

func (s *suite) m2WindowsRungs(ctx context.Context, repo string) error {
	if err := m2NativeTestsDisabled(repo); err != nil {
		return err
	}
	env, err := s.m2DatabaseEnvironment(ctx, repo, s.env, false)
	if err != nil {
		return err
	}
	env, err = s.m2WindowsEnvironment(ctx, repo, env)
	if err != nil {
		return err
	}
	got := s.m2RunInstalled(ctx, repo, rung.PreCommit, "", nil, env)
	if err := m2JudgeNative(got, false, m2DashCheck, true); err != nil {
		return err
	}
	if err := s.m2NativeMessage(ctx, repo, false, env); err != nil {
		return err
	}
	if err := s.m2NativePush(ctx, repo, false, env); err != nil {
		return err
	}
	return m2WindowsRecordersQuiet(env)
}

func m2NativeTestsDisabled(repo string) error {
	body, err := readFile(filepath.Join(repo, policyFile))
	if err != nil {
		return err
	}
	file, err := policy.Decode(body)
	if err != nil {
		return fmt.Errorf("decode native rung policy: %w", err)
	}
	if file.Ladder == nil || file.Ladder.TestStaged == nil || *file.Ladder.TestStaged {
		return fmt.Errorf("native Windows fixture must disable staged tests: %w", ErrAcceptance)
	}
	return nil
}
