package stage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/source"
)

// TestScanAsksGitForThePolicyConfigurationOncePerSnapshot holds the staged
// scan near the git processes of M2: the policy-file checks read many
// configuration files at the index and at the base, and a first read of
// each path asked git cat-file on its own, so a repository holding its
// configuration multiplied the processes of a one-line commit. The test is
// serial because it traces the git commands through the environment.
func TestScanAsksGitForThePolicyConfigurationOncePerSnapshot(t *testing.T) {
	r := gittest.New(t)
	r.Commit("feat: base", map[string]string{
		"golaw.yml": goPolicy, "go.mod": "module example.test/app\n\ngo 1.27\n",
		"tools/go.mod": "module example.test/app/tools\n\ngo 1.27\n", "Makefile": "include golaw.mk\n",
		"golaw.mk": "ci: lint\n\nlint:\n\tgolangci-lint run ./...\n", "lefthook.yml": "pre-commit:\n  commands: {}\n",
		".golangci.yml": "version: \"2\"\n", ".gitleaks.toml": "[extend]\nuseDefault = true\n",
		".github/dependabot.yml": "version: 2\nupdates: []\n", ".github/workflows/ci.yml": "on: push\njobs: {}\n",
		"docs/adr/0001-inherited-baseline.md": "---\nmodules: []\n---\n", "scripts/lint/rules.go": "package gorules\n",
		"a/a.go": "package a\n",
	})
	r.Stage(map[string]string{"a/a.go": "package a\n\n// A comment.\n"})
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	scan(t, r, source.IndexOptions{})
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	reads := strings.Count(string(data), "trace: built-in: git cat-file --batch --buffer\n")
	if reads > 5 {
		t.Errorf("the staged scan asked git cat-file --batch %d times, want at most 5:\n%s", reads, data)
	}
}
