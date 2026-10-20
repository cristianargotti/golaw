package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

const (
	m2InterruptedMarker = "M2_INSTALL_INTERRUPTED"
	m2Testdata          = "testdata"
	m2Trimpath          = "-trimpath"
)

type m2Interruption struct {
	program string
	marker  string
	env     []string
}

func (s *suite) m2InterruptedSetup(ctx context.Context, repo string) error {
	if s.m2 == nil || !s.m2.Ready {
		return fmt.Errorf("interrupted setup requires verified shared tools: %w", ErrAcceptance)
	}
	probe, err := s.m2PrepareInterruption(ctx, repo)
	if err != nil {
		return err
	}
	receipt, err := s.m2ReceiptAt(repo, probe.env)
	if err != nil {
		return err
	}
	for _, path := range []string{receipt, probe.marker, probe.marker + ".release", probe.marker + ".done"} {
		if absenceErr := m2InterruptAbsent(path); absenceErr != nil {
			return absenceErr
		}
	}
	got := s.execute(ctx, command{
		program: probe.program, args: []string{"interrupt"}, dir: repo, env: probe.env,
	})
	if evidenceErr := m2InterruptedEvidence(got, probe.marker, receipt); evidenceErr != nil {
		return evidenceErr
	}
	env, err := s.setupM2(ctx, repo, s.env)
	if err != nil {
		return err
	}
	return s.m2RepeatSetup(ctx, repo, env)
}

func (s *suite) m2PrepareInterruption(ctx context.Context, repo string) (m2Interruption, error) {
	bin := filepath.Join(s.base, "interrupted-tools")
	env, err := s.m2PrerequisiteEnvironment(s.env, bin)
	if err != nil {
		return m2Interruption{}, err
	}
	if err := s.m2CopyInterruptedTools(ctx, repo, env, bin); err != nil {
		return m2Interruption{}, err
	}
	program := filepath.Join(bin, executable("go"))
	if err := s.m2BuildInterruptProbe(ctx, repo, program, env); err != nil {
		return m2Interruption{}, err
	}
	marker := filepath.Join(s.base, "installation-marker")
	env = append(env, "GOLAW_M2_REAL_GO="+s.goTool,
		"GOLAW_M2_BINARY="+s.binary, "GOLAW_M2_MARKER="+marker)
	return m2Interruption{program: program, marker: marker, env: env}, nil
}

func (s *suite) m2CopyInterruptedTools(ctx context.Context, repo string, env []string, bin string) error {
	tools := runner.Toolchain()
	for _, tool := range tools[:len(tools)-1] {
		body, err := readFile(filepath.Join(s.m2.Bin, executable(tool.Name)))
		if err != nil {
			return err
		}
		path := filepath.Join(bin, executable(tool.Name))
		if err := writeFile(path, body, execMode); err != nil {
			return err
		}
		got := s.execute(ctx, command{program: path, args: tool.Probe, dir: repo, env: env})
		if err := success(got); err != nil {
			return err
		}
		if !tool.Matches(got.stdout) {
			return fmt.Errorf("interruption tool %s does not match %s: %w", tool.Name, tool.Version, ErrAcceptance)
		}
	}
	return m2InterruptAbsent(filepath.Join(bin, executable(tools[len(tools)-1].Name)))
}

func (s *suite) m2BuildInterruptProbe(ctx context.Context, repo, program string, env []string) error {
	body, err := readFile(filepath.Join(s.root, "tools", "acceptance", m2Testdata, "m2", "install_probe.go.txt"))
	if err != nil {
		return err
	}
	source := filepath.Join(s.base, "interrupt-probe", "main.go")
	if err := writeFile(source, body, fileMode); err != nil {
		return err
	}
	return success(s.execute(ctx, command{
		program: s.goTool, args: []string{goBuild, m2Trimpath, "-o", program, source},
		dir: repo, env: env,
	}))
}

func m2InterruptedEvidence(got result, marker, receipt string) error {
	if err := success(got); err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(got.stdout, "\r\n", "\n"), "\n")
	if !slices.Contains(lines, m2InterruptedMarker) {
		return fmt.Errorf("setup interruption was not confirmed: %w", ErrAcceptance)
	}
	for _, path := range []string{marker, marker + ".release", marker + ".done"} {
		body, err := readFile(path)
		if err != nil {
			return err
		}
		if len(body) == 0 {
			return fmt.Errorf("setup interruption marker is empty: %w", ErrAcceptance)
		}
	}
	return m2InterruptAbsent(receipt)
}

func m2InterruptAbsent(path string) error {
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect interruption evidence: %w", err)
	}
	return fmt.Errorf("unexpected interruption state at %s: %w", path, ErrAcceptance)
}
