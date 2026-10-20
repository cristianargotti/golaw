package rung

import (
	"os"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// On a host whose cgo is off, as go leaves it without a C compiler, -race
// needs cgo everywhere but macOS: what go answers then is no finding of the
// commit, so the step names the remedy and leaves the tests to the
// pipeline. The fake go is found through PATH, so the test is serial.
func TestTestStepNamesARaceDetectorThisHostLacks(t *testing.T) {
	dir := t.TempDir()
	runnertest.Install(t, dir, "go", runnertest.Behavior{Probe: []string{"version"}, Stderr: "go: -race requires cgo; enable cgo by setting CGO_ENABLED=1\n", Code: 2})
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	p := &precommit{ladder: model.Ladder{TestStaged: true}}
	tree := goTree{set: runner.AffectedSet{Packages: []string{"example.test/m/a"}, Patterns: []string{"./a"}}, dir: t.TempDir()}
	out := p.test(t.Context(), tree)
	if out.Code != codeClean || len(out.Lines) != 1 || !strings.Contains(out.Lines[0], "install a C compiler or switch test_staged off in golaw.yml") {
		t.Errorf("a race detector this host lacks = %+v", out)
	}
}

// A go test that fails without a FAIL line names no package the commit
// touches, so nothing places its failure in the commit: it is named and
// left to the pipeline. The fake go is found through PATH.
func TestTestStepLeavesToThePipelineAFailureNoPackageNames(t *testing.T) {
	dir := t.TempDir()
	runnertest.Install(t, dir, "go", runnertest.Behavior{Probe: []string{"version"}, Stderr: "go: updates to go.mod needed; to update it:\n\tgo mod tidy\n", Code: 1})
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	p := &precommit{ladder: model.Ladder{TestStaged: true}}
	tree := goTree{set: runner.AffectedSet{Packages: []string{"example.test/m/a"}, Patterns: []string{"./a"}}, dir: t.TempDir()}
	out := p.test(t.Context(), tree)
	if out.Code != codeClean || len(out.Lines) != 3 || !strings.HasSuffix(out.Lines[0], "go test -race fails without naming a package, which the pipeline judges") {
		t.Errorf("a failure no package names = %+v", out)
	}
}
