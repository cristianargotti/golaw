package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

const (
	m2EnvGoBin   = "GOBIN"
	m2JSONFlag   = "--json"
	m2GitCommand = "git"
	m2RevParse   = "rev-parse"
)

func (s *suite) m2PrerequisiteEnvironment(env []string, bin string) ([]string, error) {
	shell, err := s.lookup("sh")
	if err != nil {
		return nil, fmt.Errorf("locate M2 shell prerequisite: %w", err)
	}
	if shellErr := m2RequireExecutable(shell); shellErr != nil {
		return nil, shellErr
	}
	dirs, err := s.m2PrerequisiteDirectories(shell, bin)
	if err != nil {
		return nil, err
	}
	env = slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, envPath) || strings.EqualFold(key, m2EnvGoBin)
	})
	env = append(env, pathAssignment+strings.Join(dirs, string(os.PathListSeparator)))
	if bin != "" {
		env = append(env, "GOBIN="+bin)
	}
	return env, nil
}

func (s *suite) m2PrerequisiteDirectories(shell, bin string) ([]string, error) {
	if s.m2OS == goosWindows {
		dirs := []string{filepath.Dir(s.binary), bin, filepath.Dir(s.goTool), filepath.Dir(s.git), filepath.Dir(shell)}
		return m2DistinctDirectories(dirs), nil
	}
	dir := filepath.Join(s.base, "prerequisites")
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return nil, fmt.Errorf("create M2 prerequisite directory: %w", err)
	}
	for name, source := range map[string]string{"go": s.goTool, m2GitCommand: s.git, binaryName: s.binary, "sh": shell} {
		if err := m2PrerequisiteLink(source, filepath.Join(dir, executable(name))); err != nil {
			return nil, err
		}
	}
	return m2DistinctDirectories([]string{bin, dir}), nil
}

func m2DistinctDirectories(dirs []string) []string {
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		if dir != "" && !slices.Contains(out, dir) {
			out = append(out, dir)
		}
	}
	return out
}

func m2PrerequisiteLink(source, path string) error {
	_, err := os.Lstat(path)
	if err == nil {
		target, readErr := os.Readlink(path)
		if readErr != nil || target != source {
			return fmt.Errorf("M2 prerequisite link differs from %s: %w", source, ErrAcceptance)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect M2 prerequisite link: %w", err)
	}
	if err := os.Symlink(source, path); err != nil {
		return fmt.Errorf("link M2 prerequisite: %w", err)
	}
	return nil
}

func m2MissingTools(env []string) error {
	for _, tool := range runner.Toolchain() {
		for _, dir := range filepath.SplitList(environmentValue(env, envPath)) {
			path := filepath.Join(dir, executable(tool.Name))
			_, err := os.Lstat(path)
			if err == nil {
				return fmt.Errorf("M2 clean setup already resolves %s: %w", path, ErrAcceptance)
			}
			if !os.IsNotExist(err) {
				return fmt.Errorf("inspect absent M2 tool: %w", err)
			}
		}
	}
	return nil
}

func (s *suite) m2VerifyTools(ctx context.Context, repo string, env []string, bin string) error {
	for _, tool := range runner.Toolchain() {
		path := filepath.Join(bin, executable(tool.Name))
		if err := m2RequireExecutable(path); err != nil {
			return err
		}
		got := s.execute(ctx, command{program: path, args: tool.Probe, dir: repo, env: env})
		if err := success(got); err != nil {
			return err
		}
		if !tool.Matches(got.stdout) {
			return fmt.Errorf("M2 tool %s does not report pin %s: %w", tool.Name, tool.Version, ErrAcceptance)
		}
	}
	return nil
}

func m2RequireExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("inspect M2 executable: %w", err)
	}
	const executeBits = 0o111
	if !info.Mode().IsRegular() || runtime.GOOS != goosWindows && info.Mode().Perm()&executeBits == 0 {
		return fmt.Errorf("M2 path is not executable: %s: %w", path, ErrAcceptance)
	}
	return nil
}
