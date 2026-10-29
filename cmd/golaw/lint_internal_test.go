package main

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/toolchain"
	"github.com/cristianargotti/golaw/internal/toolchaintest"
)

const lastUserFinding = `[{"code":"DL3002","column":1,"file":"Dockerfile","level":"warning","line":3,"message":"Last USER should not be root"}]`

// dockerRepository is a committed repository with the rendered policy and
// a Dockerfile whose final user is root (docker:S6471).
func dockerRepository(t *testing.T) string {
	t.Helper()
	root := repository(t, true)
	writeFile(t, filepath.Join(root, "Dockerfile"), "FROM scratch\nCOPY app /app\nUSER 0\n")
	gitRun(t, root, "add", "Dockerfile")
	gitRun(t, root, "commit", "-q", "-m", "build: add the image")
	return root
}

func installHadolint(t *testing.T, tools string) {
	t.Helper()
	tool, ok := toolchain.Lookup("hadolint")
	if !ok {
		t.Fatal("hadolint is not pinned")
	}
	toolchaintest.Install(t, tools, tool, runtime.GOOS, runtime.GOARCH, runnertest.Behavior{Stdout: lastUserFinding, Code: 1})
}

func TestLintReportsTheFindingsOfAnInstalledTool(t *testing.T) {
	t.Parallel()
	ta, tools, _ := surfaceApp(t)
	installHadolint(t, tools)
	root := dockerRepository(t)
	if code := ta.run(t.Context(), []string{"lint", "--set", "strict", "--only", "hadolint", root}); code != exitFindings {
		t.Fatalf("exit = %d: %s%s", code, ta.stdout.String(), ta.stderr.String())
	}
	if out := ta.stdout.String(); !strings.Contains(out, "Dockerfile:3:1") || !strings.Contains(out, "lint.hadolint") {
		t.Errorf("stdout = %q", out)
	}
}

func TestLintWritesTheJSONReport(t *testing.T) {
	t.Parallel()
	ta, tools, _ := surfaceApp(t)
	installHadolint(t, tools)
	root := dockerRepository(t)
	if code := ta.run(t.Context(), []string{"lint", "--json", "--set", "strict", "--only", "dockerfile", root}); code == exitClean {
		t.Fatalf("exit = %d", code)
	}
	var doc report.Document
	if err := json.Unmarshal(ta.stdout.Bytes(), &doc); err != nil {
		t.Fatalf("decode %q: %v", ta.stdout.String(), err)
	}
	found := false
	for _, f := range doc.Findings {
		found = found || f.Check == "lint.hadolint" && f.Code == "dl3002"
	}
	if !found {
		t.Errorf("findings = %+v", doc.Findings)
	}
}

func TestLintExitsTwoNamingAMissingRequiredTool(t *testing.T) {
	t.Parallel()
	ta, _, _ := surfaceApp(t)
	root := dockerRepository(t)
	if code := ta.run(t.Context(), []string{"lint", "--set", "strict", "--only", "hadolint", root}); code != exitFailure {
		t.Fatalf("exit = %d", code)
	}
	if out := ta.stdout.String(); !strings.Contains(out, "lint.hadolint") || !strings.Contains(out, "golaw setup --surface") {
		t.Errorf("stdout = %q", out)
	}
}

func TestLintOfARepositoryWithNothingToLintIsClean(t *testing.T) {
	t.Parallel()
	ta, _, _ := surfaceApp(t)
	if code := ta.run(t.Context(), []string{"lint", "--surface", repository(t, true)}); code != exitClean {
		t.Fatalf("exit = %d: %s%s", code, ta.stdout.String(), ta.stderr.String())
	}
}

func TestLintRefusesAnUnknownSet(t *testing.T) {
	t.Parallel()
	ta, _, _ := surfaceApp(t)
	if code := ta.run(t.Context(), []string{"lint", "--set", "lenient", repository(t, true)}); code != exitFailure || !strings.Contains(ta.stderr.String(), "parity or strict") {
		t.Errorf("exit = %d, stderr %q", code, ta.stderr.String())
	}
}

func TestLintAnswersWhatItCannotLoad(t *testing.T) {
	t.Parallel()
	ta, _, _ := surfaceApp(t)
	if code := ta.run(t.Context(), []string{"lint", t.TempDir()}); code != exitFailure || !strings.Contains(ta.stderr.String(), "lint: ") {
		t.Errorf("outside a repository: exit = %d, stderr %q", code, ta.stderr.String())
	}
	homeless, _, _ := surfaceApp(t)
	b := homeless.bind()
	b.env = func(string) string { return "" }
	homeless.bind = func() binding { return b }
	// The policy load resolves the same directories first, so a machine
	// without a home stops there and names the variable to set.
	if code := homeless.run(t.Context(), []string{"lint", "--set", "strict", "--only", "hadolint", dockerRepository(t)}); code != exitFailure || !strings.Contains(homeless.stderr.String(), "HOME") {
		t.Errorf("no home: exit = %d, stdout %q, stderr %q", code, homeless.stdout.String(), homeless.stderr.String())
	}
}
