package rung_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// linter is the pinned golangci-lint a fake plays, finding nothing.
var linter = runnertest.Behavior{Probe: []string{"version"}, Version: "golangci-lint has version 2.14.0 built with go1.27.2"}

// A package go vet refuses, a format verb given a value of another type,
// and the same package as go vet passes it.
const (
	vetBroken = "package a\n\nimport \"fmt\"\n\n// A formats a value.\nfunc A() string { return fmt.Sprintf(\"%d\", \"x\") }\n"
	vetClean  = "package a\n\nimport \"fmt\"\n\n// A formats a value.\nfunc A() string { return fmt.Sprint(\"x\") }\n"
)

// emptyDir reports whether a directory exists and holds nothing.
func emptyDir(t *testing.T, dir string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) == 0
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitVetsTheAffectedPackages(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{"a/a.go": vetBroken})
	broken := rung.RunPreCommit(t.Context(), deps(t, r))
	if broken.Code != 1 || !contains(broken, "golaw pre-commit: vet: go vet reports what the affected packages break; fix it before the commit") || !contains(broken, "Sprintf format %d has arg") {
		t.Errorf("a vet problem = %+v", broken)
	}
	r.Stage(map[string]string{"a/a.go": vetClean})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, "golaw pre-commit: vet: go vet found nothing in 1 affected packages") {
		t.Errorf("a clean package = %+v", out)
	}
	r.Commit("feat: add the formatter", nil)
	r.Stage(map[string]string{"a/testdata/case.txt": "input\n"})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, "vet: go vet found nothing in 1 affected packages") {
		t.Errorf("the test data of a package = %+v", out)
	}
	r.Commit("test: add a case", nil)
	r.Stage(map[string]string{"docs/notes.md": "why\n"})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, "golaw pre-commit: vet: skipped, the commit touches no Go package") {
		t.Errorf("a commit of documents = %+v", out)
	}
	r.StageDeletion("go.mod")
	r.Stage(map[string]string{"a/a.go": vetBroken})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, "vet: skipped, the commit touches no Go package") {
		t.Errorf("an index without a module = %+v", out)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitReadsTheIndexWhenTheWorktreeDiffers(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.StageOnly("a/a.go", vetClean, vetBroken)
	d := deps(t, r)
	trees := filepath.Join(d.State, "runner", "trees")
	if out := rung.RunPreCommit(t.Context(), d); out.Code != 0 || !emptyDir(t, trees) {
		t.Errorf("a staged clean package with a broken worktree = %+v", out)
	}
	r.StageOnly("a/a.go", vetBroken, vetClean)
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 1 {
		t.Errorf("a staged broken package with a clean worktree = %+v", out)
	}
	r.Stage(map[string]string{"a/a.go": vetClean})
	r.Write("a/b.go", "package a\n\nimport \"fmt\"\n\n// B formats a value.\nfunc B() string { return fmt.Sprintf(\"%d\", \"y\") }\n")
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 {
		t.Errorf("an untracked broken file beside the package = %+v", out)
	}
	r.Remove("a/b.go")
	r.Write("b/b.go", "package b\n\nimport \"example.test/m/a\"\n\n// B is the value of a.\nfunc B() string { return a.A() }\n")
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 {
		t.Errorf("an untracked package that imports the staged one = %+v", out)
	}
	r.Remove("b/b.go")
	inPlace := deps(t, r)
	if out := rung.RunPreCommit(t.Context(), inPlace); out.Code != 0 {
		t.Errorf("a worktree that holds the index = %+v", out)
	}
	if _, err := os.Stat(filepath.Join(inPlace.State, "runner")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a worktree that holds the index is read in place: %v", err)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLeavesToThePipelineATreeItCannotCopy(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Write("target.txt", "../../outside")
	link := r.Git("hash-object", "-w", "target.txt")
	r.Remove("target.txt")
	r.Git("update-index", "--add", "--cacheinfo", "120000,"+link+",a/out")
	r.StageOnly("a/a.go", vetBroken, vetClean)
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 0 || !contains(out, "golaw pre-commit: tree: the Go steps are left to the pipeline: ") || !contains(out, "a/out") || contains(out, "vet:") {
		t.Errorf("a link the copy cannot hold = %+v", out)
	}
}

// A submodule is an empty directory in the copy, as in a checkout that
// never initialized it, so the Go steps still read the staged packages.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitVetsBesideASubmodule(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Git("update-index", "--add", "--cacheinfo", "160000,"+r.Head()+",ext/sub")
	r.StageOnly("a/a.go", vetBroken, vetClean)
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 1 || !contains(out, "golaw pre-commit: vet: go vet reports what the affected packages break") || contains(out, "left to the pipeline") {
		t.Errorf("a submodule beside a staged broken package = %+v", out)
	}
}

// Git for Windows converts line endings by default (core.autocrlf): the
// worktree holds CRLF, the index LF, and git status calls the file clean.
// A formatter reading it in place would refuse every line, so the Go
// steps read a copy of the index the commit records.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitReadsTheIndexUnderConvertedLineEndings(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	converted := []string{"GIT_CONFIG_VALUE_0=true"}
	r.Write("a/a.go", strings.ReplaceAll(vetClean, "\n", "\r\n"))
	r.GitEnv(converted, "add", "a/a.go")
	d := deps(t, r, converted...)
	trees := filepath.Join(d.State, "runner", "trees")
	if out := rung.RunPreCommit(t.Context(), d); out.Code != 0 || !emptyDir(t, trees) {
		t.Errorf("a staged file whose worktree holds CRLF = %+v", out)
	}
}

// The package graph is the commit's: a go.mod the worktree breaks after it
// was staged and a dependency the worktree changes never reach the steps,
// which read a copy of the index.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitNamesThePackagesOfTheIndex(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Commit("feat: add the value", map[string]string{"c/c.go": "package c\n\n// C is a value.\nfunc C() string { return \"c\" }\n"})
	r.Stage(map[string]string{"a/a.go": "package a\n\nimport \"example.test/m/c\"\n\n// A is the value of c.\nfunc A() string { return c.C() }\n"})
	r.Write("go.mod", "module example.test/m\n\ngo 1.27\n\nrequire (\n")
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, "vet: go vet found nothing in 1 affected packages") {
		t.Errorf("a go.mod the worktree breaks after it was staged = %+v", out)
	}
	r.Write("go.mod", "module example.test/m\n\ngo 1.27\n")
	r.Write("c/c.go", "package c\n\n// C is a value.\nfunc C() int { return 1 }\n")
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, "vet: go vet found nothing in 1 affected packages") {
		t.Errorf("a dependency the worktree changes after its importer was staged = %+v", out)
	}
}
