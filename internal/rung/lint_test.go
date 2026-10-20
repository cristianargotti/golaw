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

// complaining is the fake golangci-lint answering 1 with what it found, for
// the format and the linters alike.
func complaining(output string) runnertest.Behavior {
	b := linter
	b.Code, b.Stdout = 1, output
	return b
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitFormatsAndLintsTheStagedGo(t *testing.T) {
	dir := fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{"a/a.go": vetClean})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 0 || !contains(out, "golaw pre-commit: fmt: golangci-lint fmt finds the 1 staged Go files formatted") ||
		!contains(out, "golaw pre-commit: lint: golangci-lint found no issue on the staged lines of 1 affected packages") {
		t.Errorf("a formatted package = %+v", out)
	}
	calls := runnertest.Calls(t, dir, "golangci-lint")
	patch := filepath.Join(r.GitDir(), "golaw", "pre-commit.patch")
	want := [][]string{{"version"}, {"fmt", "--diff", filepath.FromSlash("a/a.go")}, {"run", "--allow-parallel-runners", "--new-from-patch=" + patch, "./a"}}
	if !slices.EqualFunc(calls, want, slices.Equal) {
		t.Errorf("golangci-lint calls = %v, want %v", calls, want)
	}
	if _, err := os.Stat(patch); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the staged patch stays after the run: %v", err)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitRefusesWhatGolangciLintFinds(t *testing.T) {
	r := armedRepo(t, armedPolicy)
	r.Stage(map[string]string{"a/a.go": vetClean})
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": complaining("a/a.go:6:1: exported: comment on exported function\n")})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 1 || !contains(out, "fmt: golangci-lint fmt would change the staged Go files as whole files; the lint step judges the formatters on the staged lines, the pipeline the rest") ||
		!contains(out, "lint: golangci-lint reports issues on the staged lines; fix them before the commit") || !contains(out, "a/a.go:6:1: exported") ||
		out.Lines[len(out.Lines)-1] != "golaw recusou o hook pre-commit; a causa e a correção estão nas linhas acima." {
		t.Errorf("issues = %+v", out)
	}
	broken := linter
	broken.Code, broken.Stderr = 3, "level=error msg=\"no go files to analyze\"\n"
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": broken})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 2 || !contains(out, "golangci-lint failed with exit status 3") {
		t.Errorf("a golangci-lint failure = %+v", out)
	}
	older := linter
	older.Version = "golangci-lint has version 2.13.1 built with go1.27.0"
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": older})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 2 || !contains(out, "fmt and lint: verify golangci-lint: pinned tool at another version") {
		t.Errorf("golangci-lint at another version = %+v", out)
	}
}

//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLeavesTheLintOfAMergeToThePipeline(t *testing.T) {
	dir := fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Branch("side", map[string]string{"b/b.go": "package b\n"})
	r.Commit("feat: add the formatter", map[string]string{"a/a.go": vetClean})
	r.MergeInProgress("side")
	r.Stage(map[string]string{"a/a.go": strings.Replace(vetClean, "\"x\"", "\"z\"", 1)})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 0 || !contains(out, "lint: skipped, a merge is in progress") || !contains(out, "fmt: golangci-lint fmt finds the 1 staged Go files formatted") {
		t.Errorf("a merge in progress = %+v", out)
	}
	for _, call := range runnertest.Calls(t, dir, "golangci-lint") {
		if call[0] == "run" {
			t.Errorf("golangci-lint linted a merge: %v", call)
		}
	}
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": complaining("--- a/a.go.orig\n+++ a/a.go\n")})
	whole := rung.RunPreCommit(t.Context(), deps(t, r))
	if whole.Code != 0 || !contains(whole, "fmt: golangci-lint fmt would change the staged Go files as whole files; the lint step judges the formatters on the staged lines, the pipeline the rest") {
		t.Errorf("a whole-file diff of golangci-lint fmt = %+v", whole)
	}
}

// A change of go.mod alone makes every package affected (section 7), so the
// linters run over ./... while the format step has no Go file to read.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLintsAModuleOnlyChange(t *testing.T) {
	dir := fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy)
	r.Commit("feat: add the formatter", map[string]string{"a/a.go": vetClean})
	r.Stage(map[string]string{"go.mod": "module example.test/m\n\ngo 1.27\n\n// the module of the tests\n"})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 0 || !contains(out, "golaw pre-commit: fmt: skipped, no Go file staged") ||
		!contains(out, "golaw pre-commit: lint: golangci-lint found no issue on the staged lines of 1 affected packages") {
		t.Errorf("a module-only change = %+v", out)
	}
	patch := filepath.Join(r.GitDir(), "golaw", "pre-commit.patch")
	want := [][]string{{"version"}, {"run", "--allow-parallel-runners", "--new-from-patch=" + patch, "./..."}}
	if calls := runnertest.Calls(t, dir, "golangci-lint"); !slices.EqualFunc(calls, want, slices.Equal) {
		t.Errorf("golangci-lint calls = %v, want %v", calls, want)
	}
}
