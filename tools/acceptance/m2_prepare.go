package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/install"
)

const m2CandidateVersion = "0.2.0"

type m2Shared struct {
	Base  string
	Bin   string
	Ready bool
	Live  bool
}

func (s *suite) prepareM2(ctx context.Context) error {
	s.m2 = &m2Shared{Base: s.base, Bin: filepath.Join(s.base, dirTools)}
	s.m2OS = runtime.GOOS
	s.env = m2Environment(s.env, s.home, filepath.Dir(s.binary), s.m2.Base)
	if err := s.tools(); err != nil {
		return err
	}
	binary, err := s.versionedBinary(ctx, m2CandidateVersion)
	if err != nil {
		return err
	}
	body, err := readFile(binary)
	if err != nil {
		return err
	}
	s.digest = install.Digest(body)
	return nil
}

func (s *suite) versionedBinary(ctx context.Context, version string) (string, error) {
	if !buildinfo.IsRelease(version) {
		return "", fmt.Errorf("candidate version must be X.Y.Z: %w", ErrAcceptance)
	}
	got := s.execute(ctx, command{
		program: s.git, args: []string{"-c", "core.quotePath=false", "ls-files", "-z"},
		dir: s.root, env: s.env,
	})
	if err := success(got); err != nil {
		return "", err
	}
	paths := strings.Split(strings.TrimSuffix(got.stdout, "\x00"), "\x00")
	proxy, err := writeM2Proxy(s.root, filepath.Join(s.base, "proxy"), version, paths)
	if err != nil {
		return "", err
	}
	if err := s.installM2Candidate(ctx, version, proxy); err != nil {
		return "", err
	}
	return s.binary, nil
}

func (s *suite) installM2Candidate(ctx context.Context, version, proxy string) error {
	env := append(slices.Clone(s.env),
		"GOBIN="+filepath.Dir(s.binary), "GOPROXY="+proxy+",https://proxy.golang.org,direct",
		"GONOSUMDB=github.com/cristianargotti/golaw", "GOSUMDB=sum.golang.org")
	got := s.execute(ctx, command{
		program: s.goTool,
		args:    []string{"install", "github.com/cristianargotti/golaw/cmd/golaw@v" + version},
		dir:     s.root, env: env,
	})
	if err := success(got); err != nil {
		return err
	}
	got = s.execute(ctx, command{
		program: s.binary, args: []string{"version"}, dir: s.root, env: s.env,
	})
	if err := success(got); err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(got.stdout, "\r\n", "\n")), "\n")
	if lines[0] != "golaw "+version ||
		!slices.Contains(lines, "build: source build, executable not verifiable") {
		return fmt.Errorf("candidate version or source-build limitation missing: %w", ErrAcceptance)
	}
	return nil
}
