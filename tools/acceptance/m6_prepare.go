package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/toolchain"
)

// The command that installs the surface tools, the directory of the data
// home they share and of the repository that adopts them, and the variable
// that points the candidate at that data home.
const (
	m6SetupCommand = "setup"
	m6SurfaceAll   = "--surface=all"
	m6DataDir      = "surface-data"
	m6ToolsRepo    = "surface-tools"
	envDataHome    = "XDG_DATA_HOME"
)

// prepareM6 builds the M6 candidate at its version behind the local module
// proxy, installs the pinned Go tools once, as the M3 and M5 rows do, and
// leaves the environment every case starts from.
func (s *suite) prepareM6(ctx context.Context) error {
	if err := s.prepareCandidate(ctx, m6CandidateVersion); err != nil {
		return err
	}
	proxy, err := m2FileURL(filepath.Join(s.base, m5ProxyDir))
	if err != nil {
		return err
	}
	parent := slices.Clone(s.env)
	repo, err := s.m3InstallTools(ctx)
	if err != nil {
		return err
	}
	env, err := s.m2DatabaseEnvironment(ctx, repo, prependPath(parent, s.m2.Bin), false)
	if err != nil {
		return err
	}
	s.m2.Overlay = m3Overlay(env, s.m2.Bin, proxy)
	s.env = withOverlay(env, s.m2.Overlay)
	return nil
}

// m6Scope is the isolated home of one M6 case, sharing the run's state.
func (s *m6Suite) m6Scope(category, name string) *m6Suite {
	return &m6Suite{suite: s.m3Scope(category + "-" + name), shared: s.shared}
}

// m6WithData points env at the data home data, whatever the case of the
// key an earlier entry spells, so the candidate resolves the shared tools.
func m6WithData(env []string, data string) []string {
	out := slices.DeleteFunc(slices.Clone(env), func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, envDataHome)
	})
	return append(out, envDataHome+"="+data)
}

// m6ToolEnv is the scope's environment over the shared data home.
func (s *m6Suite) m6ToolEnv() []string {
	return m6WithData(s.env, s.shared.data)
}

// m6Tools installs the surface tools once per run and answers the cause
// of a failed install to every later row without trying again.
func (s *m6Suite) m6Tools(ctx context.Context) error {
	if !s.shared.tried {
		s.shared.tried = true
		s.shared.err = s.m6InstallTools(ctx)
	}
	return s.shared.err
}

// m6InstallTools adopts a repository of its own and runs golaw setup
// --surface=all in it, the one place an M6 row reaches the network, as the
// M3 preparation does for the Go tools.
func (s *m6Suite) m6InstallTools(ctx context.Context) error {
	repo := filepath.Join(s.shared.base, m6ToolsRepo)
	if err := writeFile(filepath.Join(repo, moduleFile), []byte(m5Module), fileMode); err != nil {
		return err
	}
	if err := s.gitSteps(ctx, repo, []string{commandInit, m2Quiet}); err != nil {
		return err
	}
	if err := success(s.m5Golaw(ctx, repo, commandInit, m5CapabilitiesFlag, "go", flagApply)); err != nil {
		return fmt.Errorf("adopt the surface tools repository: %w", err)
	}
	env, err := s.m2PrerequisiteEnvironment(s.m6ToolEnv(), s.m2.Bin)
	if err != nil {
		return err
	}
	env = s.m6WithRuntimes(ctx, env)
	got := s.execute(ctx, command{program: s.binary, args: []string{m6SetupCommand, m6SurfaceAll}, dir: repo, env: env})
	if err := success(got); err != nil {
		return fmt.Errorf("install the surface tools: %w", err)
	}
	return nil
}

// m6WithRuntimes appends to the PATH of env the first directory of the
// caller's PATH whose node, and then whose python3, answers within the bounds
// of the kit's toolchain table: setup installs the npm and PyPI tools on the
// person's own runtimes, the kit never installs either one, and a version
// manager shim or an old system Python ahead of a usable one is passed over.
// A runtime the caller lacks stays absent, so setup reports it as missing.
func (s *m6Suite) m6WithRuntimes(ctx context.Context, env []string) []string {
	caller := filepath.SplitList(environmentValue(s.env, envPath))
	out := slices.Clone(env)
	for _, rt := range toolchain.Runtimes() {
		for _, dir := range caller {
			probe := toolchain.Resolver{GOOS: runtime.GOOS, Env: []string{pathAssignment + dir}}
			if dir == "" || !probe.Runtime(ctx, rt).OK {
				continue
			}
			for i, entry := range out {
				key, value, found := strings.Cut(entry, "=")
				if found && strings.EqualFold(key, envPath) {
					out[i] = pathAssignment + value + string(os.PathListSeparator) + dir
				}
			}
			break
		}
	}
	return out
}
