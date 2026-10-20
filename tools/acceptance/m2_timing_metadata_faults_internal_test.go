package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestM2TimingMetadataRejectsUnreadableVersionProbes(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, 2+len(runner.Toolchain()))
	names = append(names, "go", m2GitCommand)
	for _, tool := range runner.Toolchain() {
		names = append(names, tool.Name)
	}
	for _, name := range names {
		for _, mode := range []string{"execution", "exit", "empty", "invalid"} {
			s := m2MetadataSuite(t)
			fixture := s.execute
			reached := false
			s.execute = func(ctx context.Context, c command) result {
				if m2MetadataVersionProbe(c, name) {
					reached = true
					return m2InvalidVersionResult(name, mode)
				}
				return fixture(ctx, c)
			}
			if _, err := s.timingMetadataM2(t.Context()); err == nil || !reached {
				t.Fatalf("%s %s: reached=%t error=%v", name, mode, reached, err)
			}
		}
	}
}

func m2MetadataVersionProbe(c command, name string) bool {
	if strings.TrimSuffix(filepath.Base(c.program), ".exe") != name {
		return false
	}
	args := []string{"version"}
	if name == m2GitCommand {
		args = []string{"--version"}
	}
	if tool, found := runner.LookupTool(name); found {
		args = tool.Probe
	}
	return slices.Equal(c.args, args)
}

func m2InvalidVersionResult(name, mode string) result {
	switch mode {
	case "execution":
		return result{err: ErrAcceptance}
	case "exit":
		return result{code: 2}
	case "invalid":
		return result{stdout: name + " version not-a-version\n"}
	}
	return result{}
}

func TestM2TimingMetadataRequiresToolExecutables(t *testing.T) {
	t.Parallel()
	for _, tool := range runner.Toolchain() {
		s := m2MetadataSuite(t)
		if err := os.Remove(filepath.Join(s.m2.Bin, executable(tool.Name))); err != nil {
			t.Fatal(err)
		}
		if _, err := s.timingMetadataM2(t.Context()); err == nil {
			t.Fatalf("absent %s executable passed", tool.Name)
		}
	}
}

func TestM2TimingVersionRecognizesReleaseTokens(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value string
		valid       bool
	}{
		{"go", "go1.27.0", true},
		{"go", "go1.27rc1", true},
		{"go", "go1.27.0/binary", false},
		{"go", "unknown", false},
		{"git", "2.50.1", true},
		{"git", "2.50.1.windows.1", true},
		{"git", "2.50.1.GIT", true},
		{"git", "2.50.1/binary", false},
		{"git", "unknown", false},
	} {
		if got := m2TimingVersionValid(tc.name, tc.value); got != tc.valid {
			t.Fatalf("%s %q: valid=%t", tc.name, tc.value, got)
		}
	}
}
