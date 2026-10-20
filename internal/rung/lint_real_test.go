package rung_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// The configurations of the real golangci-lint cases: the committed one
// lints the comments, one an unstaged edit weakens, one golangci-lint
// cannot load, and a stricter one that also refuses a misspelled word.
const (
	lintComments = "version: \"2\"\nlinters:\n  default: none\n  enable: [godot]\n"
	lintWeaker   = "version: \"2\"\nlinters:\n  default: none\n  enable: [misspell]\n"
	lintBroken   = "version: \"2\"\nlinters:\n  default: none\n  enable: [nosuchlinter]\n"
	lintStricter = "version: \"2\"\nlinters:\n  default: none\n  enable: [godot, misspell]\n"
)

// realLinter skips a case of the real golangci-lint where the pinned one is
// not on PATH, which make ci installs before the tests, and fails it in CI.
func realLinter(t *testing.T) {
	t.Helper()
	tool, ok := runner.LookupTool("golangci-lint")
	if !ok {
		t.Fatal("golangci-lint is not a pinned tool")
	}
	if _, err := runner.Verify(t.Context(), tool, os.Environ()); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("CI installs golangci-lint %s before the tests: %v", tool.Version, err)
		}
		t.Skipf("golangci-lint %s is not on PATH, which make ci installs: %v", tool.Version, err)
	}
}

// golangci-lint reads the configuration of the tree it runs in, so an
// unstaged edit of a root .golangci.yml, in a module whose root holds no Go
// file, sends the steps to the copy of the index, which holds the committed
// configuration: a weaker edit hides nothing the commit introduces, and a
// broken or stricter one refuses nothing the committed one lets go.
//
//nolint:paralleltest // the fake gitleaks is found through PATH
func TestPreCommitLintsUnderTheConfigurationTheCommitRecords(t *testing.T) {
	realLinter(t)
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	r := armedRepo(t, armedPolicy)
	r.Commit("build: lint the comments", map[string]string{".golangci.yml": lintComments, "a/a.go": "package a\n\n// A answers one.\nfunc A() int { return 1 }\n"})
	r.Stage(map[string]string{"a/b.go": "package a\n\n// B answers two\nfunc B() int { return 2 }\n"})
	for name, config := range map[string]string{"the committed configuration": lintComments, "an unstaged weaker one": lintWeaker} {
		r.Write(".golangci.yml", config)
		if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 1 || !contains(out, filepath.FromSlash("a/b.go")+":3:1: Comment should end in a period (godot)") {
			t.Errorf("a staged comment without its period under %s = %+v", name, out)
		}
	}
	r.Stage(map[string]string{"a/b.go": "package a\n\n// B answers two.\nfunc B() int {\n\t// recieve the count\n\treturn 2\n}\n"})
	for name, config := range map[string]string{"an unstaged broken configuration": lintBroken, "an unstaged stricter one": lintStricter} {
		r.Write(".golangci.yml", config)
		if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, "lint: golangci-lint found no issue on the staged lines of 1 affected packages") {
			t.Errorf("a staged change the committed configuration lets go, under %s = %+v", name, out)
		}
	}
}

// golangci-lint places an issue on a staged line only through the patch
// path it can match against the file it read, so each guard of the lint
// step is a false clean or a false refusal when it goes: the tree named by
// its real path, as the directory and as PWD, for a person who reached the
// worktree through a link; the patch paths written unquoted, for a name
// git would escape; and the renames found when the person's git turns them
// off, so a moved file adds none of its old lines. A host that makes no
// link, as Windows without the right to make one, judges the other two.
//
//nolint:paralleltest // the fake gitleaks is found through PATH
func TestPreCommitPlacesTheStagedLinesThroughALinkAnEscapedNameAndARename(t *testing.T) {
	realLinter(t)
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	r := armedRepo(t, armedPolicy)
	r.Git("config", "diff.renames", "false")
	r.Commit("build: lint the comments", map[string]string{".golangci.yml": lintComments, "a/old.go": "package a\n\n// Old answers one\nfunc Old() int { return 1 }\n"})
	r.StageRename("a/old.go", "a/moved.go")
	r.Stage(map[string]string{"a/café.go": "package a\n\n// Cafe answers two\nfunc Cafe() int { return 2 }\n"})
	dir, pwd := r.Root, []string(nil)
	if link := filepath.Join(t.TempDir(), "link"); os.Symlink(r.Root, link) == nil {
		dir, pwd = link, []string{"PWD=" + link}
	}
	through := deps(t, r, pwd...)
	through.Dir = dir
	out := rung.RunPreCommit(t.Context(), through)
	if out.Code != 1 || !contains(out, filepath.FromSlash("a/café.go")+":3:1: Comment should end in a period (godot)") || contains(out, "moved.go:") {
		t.Errorf("a new issue in a staged file beside an old one a rename moved, through a link = %+v", out)
	}
}

// golangci-lint takes a lock in the temporary directory for its whole run,
// and a second run that finds it held fails at once, so another
// golangci-lint, such as the one an editor runs on save, would fail the
// lint step of a commit that introduces nothing. The plain run under the
// held lock shows the lock holds; the rung lints beside it.
//
//nolint:paralleltest // the fake gitleaks is found through PATH
func TestPreCommitLintsBesideAnotherGolangciLint(t *testing.T) {
	realLinter(t)
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks})
	r := armedRepo(t, armedPolicy)
	r.Commit("build: lint the comments", map[string]string{".golangci.yml": lintComments, "a/a.go": "package a\n\n// A answers one.\nfunc A() int { return 1 }\n"})
	r.Stage(map[string]string{"a/b.go": "package a\n\n// B answers two.\nfunc B() int { return 2 }\n"})
	tmp := t.TempDir()
	held, err := runner.Acquire(t.Context(), filepath.Join(tmp, "golangci-lint.lock"), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if released := held.Release(); released != nil {
			t.Error(released)
		}
	})
	env := []string{"TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp}
	plain, err := runner.Run(t.Context(), runner.Command{Name: "golangci-lint", Args: []string{"run", "./..."}, Dir: r.Root, Env: append(gittest.Isolated(), env...)})
	if err != nil || plain.Code == 0 || !strings.Contains(string(plain.Stdout)+string(plain.Stderr), "parallel golangci-lint is running") {
		t.Fatalf("the held lock does not stop a plain golangci-lint run: %+v, %v", plain, err)
	}
	if out := rung.RunPreCommit(t.Context(), deps(t, r, env...)); out.Code != 0 || !contains(out, "lint: golangci-lint found no issue on the staged lines of 1 affected packages") {
		t.Errorf("a clean commit beside another golangci-lint = %+v", out)
	}
}
