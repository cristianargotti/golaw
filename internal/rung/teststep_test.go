package rung_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// spawning is a staged test that runs git in a directory of its own and
// requires the answer to be about that directory, which a GIT_DIR the hook
// leaked would turn into the repository of the hook.
const spawning = `package a

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitAnswersForItsOwnDirectory(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--absolute-git-dir").Output()
	if err != nil || strings.TrimSpace(string(out)) != filepath.Join(dir, ".git") {
		t.Fatalf("git answered %q, %v for %s", out, err, dir)
	}
}
`

// failing is a staged test that fails with a line the rung repeats.
const failing = "package a\n\nimport \"testing\"\n\nfunc TestFails(t *testing.T) { t.Fatal(\"staged failure reached\") }\n"

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitTestsTheStagedPackagesOnlyWhenTheLadderSaysSo(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{"a/a.go": vetClean, "a/a_test.go": spawning})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, "golaw pre-commit: test: skipped, test_staged is off in the ladder of golaw.yml") {
		t.Errorf("test_staged off = %+v", out)
	}
	r.Write(policy.FileName, armedPolicy+"ladder: {test_staged: true}\n")
	hooked := deps(t, r, "GIT_DIR="+r.GitDir(), "GIT_WORK_TREE="+r.Root, "GIT_PREFIX=a/")
	if out := rung.RunPreCommit(t.Context(), hooked); out.Code != 0 || !contains(out, "golaw pre-commit: test: go test -race passes in 1 affected packages") {
		t.Errorf("a staged test that spawns git = %+v", out)
	}
	r.StageOnly("a/a_test.go", failing, spawning)
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 1 || !contains(out, "golaw pre-commit: test: go test -race fails in the affected packages; fix the tests before the commit") || !contains(out, "staged failure reached") {
		t.Errorf("a staged failing test = %+v", out)
	}
	docs := armedRepo(t, armedPolicy)
	docs.Write(policy.FileName, armedPolicy+"ladder: {test_staged: true}\n")
	docs.Stage(map[string]string{"docs/notes.md": "why\n"})
	if out := rung.RunPreCommit(t.Context(), deps(t, docs)); out.Code != 0 || !contains(out, "golaw pre-commit: test: skipped, the commit touches no Go package") {
		t.Errorf("test_staged over a commit of documents = %+v", out)
	}
}
