package rung_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/rung"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// A test of a package the commit touches that already fails at HEAD failed
// before the commit: a comment edit beside it lets the commit go and names
// the inherited failure, while a commit that breaks a test HEAD passes, or
// adds a failing one beside the inherited failure, is refused.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLeavesToThePipelineATestThatFailsAtHead(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	passing := "package a\n\nimport \"testing\"\n\nfunc TestPasses(t *testing.T) {\n\tif A() != \"x\" {\n\t\tt.Fatal(\"broken by the commit\")\n\t}\n}\n"
	r := armedRepo(t, armedPolicy+"ladder: {test_staged: true}\n")
	r.Commit("test: add the tests", map[string]string{"a/a.go": vetClean, "a/a_test.go": failing, "a/b_test.go": passing})
	r.Stage(map[string]string{"a/a.go": strings.Replace(vetClean, "// A formats a value.", "// A formats the value.", 1)})
	inherited := "golaw pre-commit: test: go test -race fails at HEAD too in the packages the commit touches, which the pipeline judges: example.test/m/a (TestFails)"
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, inherited) || !contains(out, "staged failure reached") {
		t.Errorf("a comment edit beside a test that fails at HEAD = %+v", out)
	}
	r.Stage(map[string]string{"a/a.go": strings.Replace(vetClean, "\"x\"", "\"y\"", 1)})
	out := rung.RunPreCommit(t.Context(), deps(t, r))
	if out.Code != 1 || !contains(out, "golaw pre-commit: test: go test -race fails in the affected packages; fix the tests before the commit") || !contains(out, "broken by the commit") {
		t.Errorf("a commit that breaks a test HEAD passes = %+v", out)
	}
	r.Stage(map[string]string{"a/a.go": vetClean, "a/c_test.go": strings.Replace(failing, "TestFails", "TestAlsoFails", 1)})
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 1 || !contains(out, "fix the tests before the commit") {
		t.Errorf("a new failing test beside the inherited one = %+v", out)
	}
}
