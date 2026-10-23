package stage_test

import (
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/source"
)

// configuration is a repository that holds the configuration the policy
// family reads, beside one Go file a commit edits.
func configuration() map[string]string {
	return map[string]string{
		"golaw.yml": goPolicy, "go.mod": "module example.test/app\n\ngo 1.27\n",
		"tools/go.mod": "module example.test/app/tools\n\ngo 1.27\n", "Makefile": "include golaw.mk\n",
		"golaw.mk": "ci: lint\n\nlint:\n\tgolangci-lint run ./...\n", "lefthook.yml": "pre-commit:\n  commands: {}\n",
		".golangci.yml": "version: \"2\"\n", ".gitleaks.toml": "[extend]\nuseDefault = true\n",
		".github/dependabot.yml": "version: 2\nupdates: []\n", ".github/workflows/ci.yml": "on: push\njobs: {}\n",
		"docs/adr/0001-inherited-baseline.md": "---\nmodules: []\n---\n", "scripts/lint/rules.go": "package gorules\n",
		"a/a.go": "package a\n",
	}
}

// catFileReads commits the files, stages a comment in a/a.go, runs the
// staged scan and answers how many times it asked git cat-file --batch,
// from the trace file the caller set GIT_TRACE to, emptied before the scan.
func catFileReads(t *testing.T, files map[string]string, trace string) (int, string) {
	t.Helper()
	r := gittest.New(t)
	r.Commit("feat: base", files)
	r.Stage(map[string]string{"a/a.go": "package a\n\n// A comment.\n"})
	if err := os.WriteFile(trace, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	scan(t, r, source.IndexOptions{})
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "trace: built-in: git cat-file --batch --buffer\n"), string(data)
}

// TestScanAsksGitForThePolicyConfigurationOncePerSnapshot holds the staged
// scan near the git processes of M2: the policy-file checks read many
// configuration files at the index and at the base, and a first read of
// each path asked git cat-file on its own, so a repository holding its
// configuration multiplied the processes of a one-line commit. The test is
// serial because it traces the git commands through the environment.
func TestScanAsksGitForThePolicyConfigurationOncePerSnapshot(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	if reads, data := catFileReads(t, configuration(), trace); reads > 5 {
		t.Errorf("the staged scan asked git cat-file --batch %d times, want at most 5:\n%s", reads, data)
	}
}

// TestScanOfALargeTreeAsksGitAsASmallOne is the large-tree row: thousands
// of files beside the configuration add no git question, and the prefetch
// walks the tree once per snapshot, not once per configuration read, which
// made a 60,000 file repository twice as slow as before the prefetch.
// Serial for the same reason as the row above.
func TestScanOfALargeTreeAsksGitAsASmallOne(t *testing.T) {
	files := configuration()
	assets := make(map[string]string, 3000)
	for i := range 3000 {
		assets["assets/"+strconv.Itoa(i/100)+"/"+strconv.Itoa(i)+".txt"] = "asset " + strconv.Itoa(i) + "\n"
	}
	maps.Copy(files, assets)
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	if reads, _ := catFileReads(t, files, trace); reads > 5 {
		t.Errorf("the staged scan of a large tree asked git cat-file --batch %d times, want at most 5", reads)
	}
}
