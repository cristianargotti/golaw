package goldentest

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/source"
)

// fakeLint is a harness whose lint steps run a fake golangci-lint, so the
// steps are proven whichever golangci-lint the machine holds or lacks.
func fakeLint(t *testing.T, b runnertest.Behavior) *harness {
	t.Helper()
	h := inGit(t)
	if err := os.WriteFile(source.OSPath(h.repo.Root, lintAsset), []byte("version: \"2\"\nlinters:\n  exclusions:\n    generated: lax\n"), fillMode); err != nil {
		t.Fatal(err)
	}
	h.report.Files = map[string][]byte{regenerated: regeneratedSource()}
	if err := os.MkdirAll(source.OSPath(h.repo.Root, "internal/x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source.OSPath(h.repo.Root, regenerated), regeneratedSource(), fillMode); err != nil {
		t.Fatal(err)
	}
	tool := runnertest.Install(t, t.TempDir(), lintTool, b)
	h.resolveLint = func(context.Context, string) (string, string) { return tool, "" }
	return h
}

// Red rows: a lint that exits 1 fails both runs and keeps the header; a
// format diff fails the file. Near rows: a clean tool fails nothing.
func TestTheLintStepsRunTheToolTheyResolve(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		behavior runnertest.Behavior
		step     func(h *harness, ctx context.Context)
		want     []string
	}{
		"red: lint findings":   {runnertest.Behavior{Stdout: "x.go:1:1: finding", Code: 1}, (*harness).lint, []string{"lint: exit 1: x.go:1:1: finding", "lint: as the adopter runs it: exit 1: "}},
		"near: a clean lint":   {runnertest.Behavior{}, (*harness).lint, nil},
		"red: a format diff":   {runnertest.Behavior{Stdout: "-a\n+b"}, (*harness).format, []string{"format: " + regenerated + " differs from golangci-lint fmt: -a"}},
		"near: a clean format": {runnertest.Behavior{}, (*harness).format, nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := fakeLint(t, tc.behavior)
			tc.step(h, t.Context())
			if len(h.failures) != len(tc.want) {
				t.Fatalf("failures = %q, want %q", h.failures, tc.want)
			}
			for i, want := range tc.want {
				if !strings.HasPrefix(h.failures[i], want) {
					t.Errorf("failure %d = %q, want it to start %q", i, h.failures[i], want)
				}
			}
			if content, err := os.ReadFile(source.OSPath(h.repo.Root, regenerated)); err != nil || !bytes.Equal(content, regeneratedSource()) {
				t.Errorf("the regenerable file after the step = %q, %v, want its header back", content, err)
			}
		})
	}
}

// A tool the harness cannot resolve skips both steps with the reason.
func TestTheLintStepsSkipWithTheReasonTheResolverGives(t *testing.T) {
	t.Parallel()
	h := fakeLint(t, runnertest.Behavior{})
	h.resolveLint = func(context.Context, string) (string, string) { return "", "golangci-lint 2.14.0 not found" }
	h.lint(t.Context())
	h.format(t.Context())
	if h.report.Skipped[StepLint] == "" || h.report.Skipped[StepFormat] == "" || len(h.failures) != 0 {
		t.Errorf("skipped %v, failures %v", h.report.Skipped, h.failures)
	}
	if path, reason := pinned(t.Context(), "nosuch"); path != "" || !strings.Contains(reason, "is not a pinned tool") {
		t.Errorf("pinned(nosuch) = %q, %q", path, reason)
	}
	if path, reason := resolveLint(t.Context(), h.repo.Root); (path == "") == (reason == "") {
		t.Errorf("resolveLint = %q, %q: a path or a reason, never both or neither", path, reason)
	}
	if cached, reason := dslCached(t.Context(), h.repo.Root); cached != (reason == "") {
		t.Errorf("dslCached = %v, %q", cached, reason)
	}
	if cached, reason := dslCached(t.Context(), t.TempDir()+"/absent"); cached || reason == "" {
		t.Errorf("dslCached outside any directory = %v, %q", cached, reason)
	}
}
