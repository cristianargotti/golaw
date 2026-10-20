package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runner"
)

const (
	m2TimingGoCache     = "GOCACHE"
	m2TimingModuleCache = "GOMODCACHE"
	m2TimingLintCache   = "GOLANGCI_LINT_CACHE"
)

func (s *suite) timingM2(ctx context.Context, rungName, cacheRoot string) (command, error) {
	if !slices.Contains(rung.Names(), rungName) || cacheRoot == "" || s.m2 == nil || !s.m2.Ready {
		return command{}, fmt.Errorf("timing requires a rung, cache root and verified tools: %w", ErrAcceptance)
	}
	root, err := filepath.Abs(cacheRoot)
	if err != nil {
		return command{}, fmt.Errorf("resolve timing cache root: %w", err)
	}
	key := rungName + "\x00" + root + "\x00" + strconv.FormatBool(s.m2.Live)
	if prepared, found := s.m2Timings[key]; found {
		return m2CopyCommand(prepared), nil
	}
	scope := s.m2Scope("timing-" + install.Digest([]byte(key)))
	prepared, err := scope.m2PrepareTiming(ctx, rungName, root)
	if err != nil {
		return command{}, err
	}
	if s.m2Timings == nil {
		s.m2Timings = make(map[string]command)
	}
	s.m2Timings[key] = prepared
	return m2CopyCommand(prepared), nil
}

func m2CopyCommand(c command) command {
	c.args, c.env = slices.Clone(c.args), slices.Clone(c.env)
	return c
}

func (s *suite) m2PrepareTiming(ctx context.Context, rungName, cacheRoot string) (command, error) {
	repo, err := s.m2TimingRepository(ctx, rungName)
	if err != nil {
		return command{}, err
	}
	env, err := s.setupM2(ctx, repo, s.env)
	if err != nil {
		return command{}, err
	}
	if rungName == rung.PrePush {
		env, err = s.m2TimingPushEnvironment(ctx, repo, env)
	} else {
		for i, entry := range env {
			if strings.HasPrefix(entry, pathAssignment) {
				env[i] = entry + string(os.PathListSeparator) + environmentValue(s.env, envPath)
			}
		}
	}
	if err != nil {
		return command{}, err
	}
	input, args, err := s.m2TimingArguments(ctx, repo, rungName, env)
	if err != nil {
		return command{}, err
	}
	return s.installedM2(ctx, repo, rungName, input, args, m2TimingCaches(env, cacheRoot))
}

func (s *suite) m2TimingArguments(ctx context.Context, repo, rungName string, env []string) (string, []string, error) {
	switch rungName {
	case rung.CommitMsg:
		path := filepath.Join(s.base, m2MessageFile)
		if err := writeFile(path, []byte(m2MessageBody), fileMode); err != nil {
			return "", nil, err
		}
		return "", []string{path}, nil
	case rung.PrePush:
		head, err := s.m2ObserveGit(ctx, repo, env, m2RevParse, m2Verify, m2Head)
		if err != nil {
			return "", nil, err
		}
		input := "refs/heads/main " + head + " refs/heads/main " + runner.ZeroSHA + "\n"
		return input, []string{"local", filepath.Join(s.base, "local-remote")}, nil
	}
	return "", nil, nil
}

func m2TimingCaches(env []string, root string) []string {
	names := map[string]string{
		m2TimingGoCache: "go-build", m2TimingModuleCache: "go-mod", m2TimingLintCache: "golangci-lint",
	}
	env = slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		_, found := names[strings.ToUpper(key)]
		return found
	})
	for _, key := range []string{m2TimingGoCache, m2TimingModuleCache, m2TimingLintCache} {
		env = append(env, key+"="+filepath.Join(root, names[key]))
	}
	return env
}

func (s *suite) m2TimingPushEnvironment(ctx context.Context, repo string, env []string) ([]string, error) {
	if !s.m2.Live {
		return s.m2PushEnvironment(ctx, repo, env, false)
	}
	bin := filepath.Join(s.base, m2PushToolsDir)
	env, err := s.m2PrerequisiteEnvironment(env, bin)
	if err != nil {
		return nil, err
	}
	if copyErr := s.m2CopyPushDispatcher(bin); copyErr != nil {
		return nil, copyErr
	}
	scanner, err := readFile(filepath.Join(s.m2.Bin, executable(m2Scanner)))
	if err != nil {
		return nil, err
	}
	if err := writeFile(filepath.Join(bin, executable(m2Scanner)), scanner, execMode); err != nil {
		return nil, err
	}
	if err := m2PushToolsAbsent(env); err != nil {
		return nil, err
	}
	return env, nil
}
