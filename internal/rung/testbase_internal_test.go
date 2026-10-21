package rung

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestFailedTestsNameTheTopLevelTestsOfEachPackage(t *testing.T) {
	t.Parallel()
	lines := []string{
		"--- FAIL: TestA (0.00s)", "    --- FAIL: TestA/sub (0.00s)", "--- FAIL: TestB (0.01s)", "FAIL", "FAIL\texample.test/m/a\t0.02s",
		"ok  \texample.test/m/b\t0.01s", "--- FAIL: TestC (0.00s)", "FAIL", "FAIL\texample.test/m/c\t0.01s", "FAIL\texample.test/m/d [build failed]",
	}
	out := runner.Output{Stdout: []byte(strings.Join(lines, "\n") + "\n")}
	got := failedTests(out)
	if !slices.Equal(got["example.test/m/a"], []string{"TestA", "TestB"}) || !slices.Equal(got["example.test/m/c"], []string{"TestC"}) ||
		len(got["example.test/m/b"]) != 0 || len(got["example.test/m/d"]) != 0 {
		t.Errorf("failedTests = %v", got)
	}
}

// A failure no test names, as a package that does not build, and a branch
// with no commit leave the refusal as it stands, without asking HEAD.
func TestHeldAtHeadComparesOnlyNamedTestsOfACommit(t *testing.T) {
	t.Parallel()
	p := &precommit{}
	built := runner.Output{Stdout: []byte("FAIL\texample.test/m/a [build failed]\n")}
	if _, ok := p.heldAtHead(t.Context(), built, []string{"example.test/m/a"}); ok {
		t.Error("a package that does not build reads as held at HEAD")
	}
	named := runner.Output{Stdout: []byte("--- FAIL: TestA (0.00s)\nFAIL\texample.test/m/a\t0.01s\n")}
	if _, ok := p.heldAtHead(t.Context(), named, []string{"example.test/m/a"}); ok {
		t.Error("a branch with no commit reads as held at HEAD")
	}
	p.repo.Head, p.deps.Dir, p.deps.State = "0123456789012345678901234567890123456789", t.TempDir(), t.TempDir()
	if _, ok := p.heldAtHead(t.Context(), named, []string{"example.test/m/a"}); ok {
		t.Error("a HEAD that cannot be copied reads as held at HEAD")
	}
}
