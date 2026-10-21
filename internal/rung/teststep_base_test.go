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

// exitingMain is a TestMain that fails the package after its tests pass,
// as a setup that needs a database or a container does, so go test names
// no failing test.
const exitingMain = "package a\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestMain(m *testing.M) {\n\tm.Run()\n\tos.Exit(1)\n}\n\nfunc TestPasses(t *testing.T) {}\n"

// A package whose TestMain already fails at HEAD, naming no test, failed
// before the commit: a comment edit beside it goes and names the package,
// while a commit that brings such a failure to a package HEAD passes is
// refused with a reason that names the off switch.
//
//nolint:paralleltest // the fake tools are found through PATH
func TestPreCommitLeavesToThePipelineAnUnnamedFailureAtHead(t *testing.T) {
	fakes(t, map[string]runnertest.Behavior{"gitleaks": leaks, "golangci-lint": linter})
	r := armedRepo(t, armedPolicy+"ladder: {test_staged: true}\n")
	r.Commit("test: add the setup", map[string]string{"a/a.go": vetClean, "a/main_test.go": exitingMain})
	r.Stage(map[string]string{"a/a.go": strings.Replace(vetClean, "// A formats a value.", "// A formats the value.", 1)})
	inherited := "golaw pre-commit: test: go test -race fails at HEAD too in the packages the commit touches, which the pipeline judges: example.test/m/a (no test named)"
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 0 || !contains(out, inherited) {
		t.Errorf("a comment edit beside a TestMain that fails at HEAD = %+v", out)
	}
	r = armedRepo(t, armedPolicy+"ladder: {test_staged: true}\n")
	r.Commit("test: add the tests", map[string]string{"a/a.go": vetClean, "a/main_test.go": strings.Replace(exitingMain, "os.Exit(1)", "os.Exit(0)", 1)})
	r.Stage(map[string]string{"a/main_test.go": exitingMain})
	refusal := "golaw pre-commit: test: go test -race fails in the affected packages; fix the tests before the commit or switch test_staged off in golaw.yml"
	if out := rung.RunPreCommit(t.Context(), deps(t, r)); out.Code != 1 || !contains(out, refusal) {
		t.Errorf("a commit that makes a TestMain fail = %+v", out)
	}
}
