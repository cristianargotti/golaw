package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	m2Scanner         = "govulncheck"
	m2RealScannerKey  = "GOLAW_M2_REAL_GOVULNCHECK"
	m2DatabaseKey     = "GOLAW_M2_DATABASE"
	m2ScannerProxyDir = "scanner-proxy"
	m2PushToolsDir    = "push-tools"
)

func (s *suite) m2DatabaseEnvironment(ctx context.Context, repo string, env []string, red bool) ([]string, error) {
	if s.m2 == nil || !s.m2.Ready {
		return nil, fmt.Errorf("scanner proxy requires verified shared tools: %w", ErrAcceptance)
	}
	scanner := filepath.Join(s.m2.Bin, executable(m2Scanner))
	if err := m2RequireExecutable(scanner); err != nil {
		return nil, err
	}
	database, err := s.m2DatabaseURL(red)
	if err != nil {
		return nil, err
	}
	bin := filepath.Join(s.base, m2ScannerProxyDir)
	if err := s.m2BuildScannerProxy(ctx, repo, filepath.Join(bin, executable(m2Scanner)), env); err != nil {
		return nil, err
	}
	env = slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, m2RealScannerKey) || strings.EqualFold(key, m2DatabaseKey) ||
			strings.EqualFold(key, "GOVULNDB")
	})
	return append(prependPath(env, bin), m2RealScannerKey+"="+scanner, m2DatabaseKey+"="+database), nil
}

func (s *suite) m2DatabaseURL(red bool) (string, error) {
	name := "empty-db"
	files := []string{"index/db.json", "index/modules.json"}
	if red {
		name = "red-db"
		files = append(files, "ID/GO-2026-0001.json")
	}
	dir := filepath.Join(s.root, dirTools, dirAcceptance, m2Testdata, "m2", name)
	for _, file := range files {
		if _, err := readFile(filepath.Join(dir, filepath.FromSlash(file))); err != nil {
			return "", err
		}
	}
	return m2FileURL(dir)
}

func (s *suite) m2BuildScannerProxy(ctx context.Context, repo, program string, env []string) error {
	body, err := readFile(filepath.Join(s.root, dirTools, dirAcceptance, m2Testdata, "m2", "govulncheck_proxy.go.txt"))
	if err != nil {
		return err
	}
	source := filepath.Join(s.base, "scanner-source", m2FixtureMain)
	if err := writeFile(source, body, fileMode); err != nil {
		return err
	}
	got := s.execute(ctx, command{
		program: s.goTool, args: []string{goBuild, m2Trimpath, "-o", program, source}, dir: repo, env: env,
	})
	if err := success(got); err != nil {
		return err
	}
	return m2RequireExecutable(program)
}

func (s *suite) m2PushEnvironment(ctx context.Context, repo string, env []string, red bool) ([]string, error) {
	bin := filepath.Join(s.base, m2PushToolsDir)
	env, err := s.m2PrerequisiteEnvironment(env, bin)
	if err != nil {
		return nil, err
	}
	if copyErr := s.m2CopyPushDispatcher(bin); copyErr != nil {
		return nil, copyErr
	}
	env, err = s.m2DatabaseEnvironment(ctx, repo, env, red)
	if err != nil {
		return nil, err
	}
	if err := m2PushToolsAbsent(env); err != nil {
		return nil, err
	}
	return env, nil
}

func (s *suite) m2CopyPushDispatcher(bin string) error {
	if s.m2 == nil || !s.m2.Ready {
		return fmt.Errorf("push isolation requires verified shared tools: %w", ErrAcceptance)
	}
	source := filepath.Join(s.m2.Bin, executable("lefthook"))
	if err := m2RequireExecutable(source); err != nil {
		return err
	}
	body, err := readFile(source)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(bin, executable("lefthook")), body, execMode)
}

func m2PushToolsAbsent(env []string) error {
	for _, name := range []string{"gitleaks", "golangci-lint"} {
		for _, dir := range filepath.SplitList(environmentValue(env, envPath)) {
			path := filepath.Join(dir, executable(name))
			_, err := os.Lstat(path)
			if err == nil {
				return fmt.Errorf("M2 push still resolves unrelated tool %s: %w", path, ErrAcceptance)
			}
			if !os.IsNotExist(err) {
				return fmt.Errorf("inspect absent push tool: %w", err)
			}
		}
	}
	return nil
}
