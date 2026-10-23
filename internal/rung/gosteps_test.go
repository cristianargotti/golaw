package rung_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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

// vetAt starts the line go vet writes for the format problem of a file at
// a slash path; go names the file with the separator of the system, so
// a/a.go reads a\a.go on Windows, and a rung prints the line as go wrote it.
func vetAt(path string) string {
	return filepath.FromSlash(path) + ":6:39: "
}

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
	// An untracked file go vet refuses, beside the staged package or in a
	// package that imports it, sends the steps to the copy of the index,
	// which holds neither: read in place, its problem would be named.
	untracked := map[string]string{
		"a/b.go": "package a\n\nimport \"fmt\"\n\n// B formats a value.\nfunc B() string { return fmt.Sprintf(\"%d\", \"y\") }\n",
		"b/b.go": "package b\n\nimport (\n\t\"fmt\"\n\n\t\"example.test/m/a\"\n)\n\n// B formats the value of a.\nfunc B() string { return fmt.Sprintf(\"%d\", a.A()) }\n",
	}
	for path, content := range untracked {
		r.Write(path, content)
		d := deps(t, r)
		out := rung.RunPreCommit(t.Context(), d)
		if out.Code != 0 || contains(out, path) || !contains(out, "vet: go vet found nothing in 1 affected packages") || !emptyDir(t, filepath.Join(d.State, "runner", "trees")) {
			t.Errorf("an untracked %s go vet refuses = %+v", path, out)
		}
		r.Remove(path)
	}
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

// Git ignores files the commit never records, yet go reads every Go file of
// the module: in place the steps would judge them, and a copy of the index
// would lack them, which refuses every commit of a repository that ignores
// the code it generates. The Go steps are left to the pipeline with the
// files named, a Go file of an ignored directory included; an ignored file
// go never reads keeps the worktree read in place.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLeavesToThePipelineTheGoFilesGitIgnores(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Commit("chore: ignore the local files", map[string]string{".gitignore": "local.go\n[._]*.go\ngen/\n*.log\n", "a/a.go": vetClean})
	r.Stage(map[string]string{"a/b.go": "package a\n\n// B answers one.\nfunc B() int { return 1 }\n"})
	for name, path := range map[string]string{"an ignored broken Go file beside the staged package": "a/local.go", "a Go file of an ignored directory": "gen/gen.go"} {
		r.Write(path, "package a\n\nimport \"fmt\"\n\n// L formats a value.\nfunc L() string { return fmt.Sprintf(\"%d\", \"x\") }\n")
		out := rung.RunPreCommit(t.Context(), deps(t, r))
		if out.Code != 0 || !contains(out, "golaw pre-commit: tree: the Go steps are left to the pipeline: ") || !contains(out, path) || contains(out, "vet:") {
			t.Errorf("%s = %+v", name, out)
		}
		r.Remove(path)
	}
	r.Write("a/run.log", "ran\n")
	r.Write("a/.local.go", "package a\n\nfunc broken(\n")
	r.Write("a/_local.go", "package a\n\nfunc broken(\n")
	inPlace := deps(t, r)
	if out := rung.RunPreCommit(t.Context(), inPlace); out.Code != 0 || !contains(out, "vet: go vet found nothing in 1 affected packages") {
		t.Errorf("ignored files go never reads = %+v", out)
	}
	if _, err := os.Stat(filepath.Join(inPlace.State, "runner")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("ignored files go never reads keep the worktree read in place: %v", err)
	}
}

// A rung refuses only what the commit introduces: a problem go vet reports
// on a line the staged patch adds, as golangci-lint keeps the issues of the
// staged lines. A problem HEAD already holds is named and left to the
// pipeline, so a clean staged file beside it lets the commit go, while the
// commit that adds a problem of its own, or edits the line of the old one,
// is refused.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitRefusesOnlyTheVetProblemsTheCommitAdds(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Commit("feat: add the formatter", map[string]string{"a/a.go": vetBroken})
	inherited := "golaw pre-commit: vet: go vet reports problems on lines the commit does not add, which the pipeline judges"
	r.Stage(map[string]string{"a/b.go": "package a\n\n// B answers one.\nfunc B() int { return 1 }\n"})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 0 || !contains(out, inherited) || !contains(out, vetAt("a/a.go")+"fmt.Sprintf format %d has arg") || contains(out, "fix it before the commit") {
		t.Errorf("a vet problem HEAD holds beside a clean staged file = %+v", out)
	}
	r.Stage(map[string]string{"a/b.go": "package a\n\nimport \"fmt\"\n\n// B formats a value.\nfunc B() string { return fmt.Sprintf(\"%d\", \"y\") }\n"})
	out = rung.RunPreCommit(t.Context(), deps(t, r))
	refusal := slices.Index(out.Lines, "golaw pre-commit: vet: go vet reports what the affected packages break; fix it before the commit")
	if out.Code != 1 || refusal < 0 || refusal+1 >= len(out.Lines) || !strings.HasPrefix(out.Lines[refusal+1], vetAt("a/b.go")) || !contains(out, inherited) {
		t.Errorf("a staged vet problem beside the one HEAD holds = %+v", out)
	}
	r.Remove("a/b.go")
	r.Git("rm", "-q", "--cached", "a/b.go")
	r.Stage(map[string]string{"a/a.go": strings.Replace(vetBroken, "\"x\"", "\"z\"", 1)})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 1 || !contains(out, vetAt("a/a.go")) || contains(out, inherited) {
		t.Errorf("a staged edit of the line of the old problem = %+v", out)
	}
}

// While a merge is in progress the staged patch also holds what the merge
// brought, so a line is the commit's own only when the index adds it
// against the other parent too: a vet problem the merge brought into a
// staged package is left to the pipeline, one the person writes is refused.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitRefusesOnlyTheVetProblemsAMergeAddsItself(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Commit("feat: add the formatter", map[string]string{"a/a.go": vetClean})
	r.Branch("side", map[string]string{"a/c.go": "package a\n\nimport \"fmt\"\n\n// C formats a value.\nfunc C() string { return fmt.Sprintf(\"%d\", \"c\") }\n"})
	r.Commit("docs: say why", map[string]string{"notes.md": "why\n"})
	r.MergeInProgress("side")
	r.Stage(map[string]string{"a/a.go": strings.Replace(vetClean, "\"x\"", "\"z\"", 1)})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 0 || !contains(out, "vet: go vet reports problems on lines the commit does not add, which the pipeline judges") || !contains(out, vetAt("a/c.go")) {
		t.Errorf("a vet problem the merge brought beside a clean edit = %+v", out)
	}
	r.Stage(map[string]string{"a/a.go": vetBroken})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 1 || !contains(out, "vet: go vet reports what the affected packages break; fix it before the commit") || !contains(out, vetAt("a/a.go")) {
		t.Errorf("a vet problem the merge commit adds itself = %+v", out)
	}
}
