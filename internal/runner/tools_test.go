package runner_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

func sameTool(a, b runner.Tool) bool {
	return a.Name == b.Name && a.Module == b.Module && a.Version == b.Version && slices.Equal(a.Probe, b.Probe)
}

func TestToolchainPinsEveryToolOfTheRungs(t *testing.T) {
	t.Parallel()
	want := []runner.Tool{
		{Name: "gitleaks", Module: "github.com/zricethezav/gitleaks/v8", Version: "8.30.1", Probe: []string{"version"}},
		{Name: "golangci-lint", Module: "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", Version: "2.14.0", Probe: []string{"version"}},
		{Name: "govulncheck", Module: "golang.org/x/vuln/cmd/govulncheck", Version: "1.7.0", Probe: []string{"-version"}},
		{Name: "lefthook", Module: "github.com/evilmartians/lefthook/v2", Version: "2.1.12", Probe: []string{"version"}},
	}
	if got := runner.Toolchain(); !slices.EqualFunc(got, want, sameTool) {
		t.Errorf("Toolchain() = %+v", got)
	}
	vuln, ok := runner.LookupTool("govulncheck")
	if !ok || !sameTool(vuln, want[2]) {
		t.Errorf("LookupTool(govulncheck) = %+v, %v", vuln, ok)
	}
	if _, ok := runner.LookupTool("make"); ok {
		t.Error("make is no tool of the rungs")
	}
	if args := vuln.InstallArgs(); !slices.Equal(args, []string{"install", "golang.org/x/vuln/cmd/govulncheck@v1.7.0"}) {
		t.Errorf("InstallArgs() = %v", args)
	}
}

func TestMatchesNamesThePinAsAWholeToken(t *testing.T) {
	t.Parallel()
	leaks, _ := runner.LookupTool("gitleaks")
	lint, _ := runner.LookupTool("golangci-lint")
	vuln, _ := runner.LookupTool("govulncheck")
	hook, _ := runner.LookupTool("lefthook")
	cases := []struct {
		tool   runner.Tool
		output string
		want   bool
	}{
		{leaks, "8.30.1\n", true},
		{leaks, "v8.30.1\n", true},
		{lint, "golangci-lint has version 2.14.0 built with go1.27.2 from (unknown)", true},
		{vuln, "Go: go1.27.0\nScanner: govulncheck@v1.7.0\nDB: https://vuln.go.dev", true},
		{hook, "2.1.12\n", true},
		{lint, "golangci-lint has version 2.14.01 built with go1.27.2", false},
		{lint, "golangci-lint has version 12.14.0 built with go1.27.2", false},
		{lint, "golangci-lint has version 2.14.0-rc1 built with go1.27.2", false},
		{vuln, "Scanner: govulncheck@v1.7.1", false},
		{hook, "2.1.120\n", false},
		{leaks, "", false},
	}
	for _, c := range cases {
		if got := c.tool.Matches(c.output); got != c.want {
			t.Errorf("%s.Matches(%q) = %v, want %v", c.tool.Name, c.output, got, c.want)
		}
	}
}

func TestVerifyFindsThePinnedToolOnThePath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	lint, _ := runner.LookupTool("golangci-lint")
	path := runnertest.Install(t, dir, "golangci-lint", runnertest.Behavior{Probe: lint.Probe, Version: "golangci-lint has version 2.14.0 built with go1.27.2"})
	if got, err := runner.Verify(t.Context(), lint, nil); err != nil || got != path {
		t.Errorf("Verify = %q, %v, want %s", got, err, path)
	}
	runnertest.Install(t, dir, "golangci-lint", runnertest.Behavior{Probe: lint.Probe, Version: "golangci-lint has version 2.13.1 built with go1.27.0"})
	_, err := runner.Verify(t.Context(), lint, nil)
	if !errors.Is(err, runner.ErrToolVersion) || !strings.Contains(err.Error(), "2.13.1") || !strings.Contains(err.Error(), "golaw setup") {
		t.Errorf("another version: %v", err)
	}
	runnertest.Install(t, dir, "golangci-lint", runnertest.Behavior{Probe: []string{"other"}, Stderr: "unknown command\n", Code: 2})
	if _, err := runner.Verify(t.Context(), lint, nil); !errors.Is(err, runner.ErrToolVersion) || !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("a probe that fails: %v", err)
	}
	vuln, _ := runner.LookupTool("govulncheck")
	if _, err := runner.Verify(t.Context(), vuln, nil); !errors.Is(err, runner.ErrToolMissing) || !strings.Contains(err.Error(), "golaw setup") {
		t.Errorf("a tool PATH lacks: %v", err)
	}
}
