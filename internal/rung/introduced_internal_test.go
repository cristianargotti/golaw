package rung

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestMarkReadsTheNewSideOfEachHunk(t *testing.T) {
	t.Parallel()
	added := map[string]map[int]bool{}
	for _, hunk := range []struct{ file, header string }{
		{"a.go", "@@ -2 +2 @@ func A() {"},
		{"a.go", "@@ -3,0 +4,2 @@"},
		{"a.go", "@@ -7,2 +8,0 @@"},
		{"", "@@ -1,3 +0,0 @@"},
		{"b.go", "@@ -1 +x,y @@"},
		{"b.go", "@@"},
	} {
		mark(added, hunk.file, hunk.header)
	}
	if want := map[string]map[int]bool{"a.go": {2: true, 4: true, 5: true}}; !maps.EqualFunc(added, want, maps.Equal) {
		t.Errorf("added lines = %v, want %v", added, want)
	}
}

func TestAttributedKeepsTheProblemsOfTheLinesTheCommitAdds(t *testing.T) {
	t.Parallel()
	added := map[string]map[int]bool{"a/b.go": {6: true}}
	out := runner.Output{Stderr: []byte("# example.test/m/a\n" +
		"a/b.go:6:39: a problem the commit adds\n\tits continuation\n" +
		"a/a.go:6:39: a problem HEAD holds\n\tits continuation\n" +
		"go: a line no position starts\n" +
		`a\b.go:6:2: a problem named the Windows way` + "\n" +
		`C:\w\a\b.go:6:2: an absolute path` + "\n")}
	kept, inherited := attributed(out, added)
	if want := []string{"a/b.go:6:39: a problem the commit adds", "\tits continuation", `a\b.go:6:2: a problem named the Windows way`}; !slices.Equal(kept, want) {
		t.Errorf("kept = %q, want %q", kept, want)
	}
	if want := []string{"a/a.go:6:39: a problem HEAD holds", "\tits continuation", "go: a line no position starts", `C:\w\a\b.go:6:2: an absolute path`}; !slices.Equal(inherited, want) {
		t.Errorf("inherited = %q, want %q", inherited, want)
	}
}

func TestFailedPackagesAreTheFailLinesOfGoTest(t *testing.T) {
	t.Parallel()
	lines := []string{"--- FAIL: TestA (0.00s)", "FAIL", "FAIL\texample.test/m/a\t0.01s", "ok  \texample.test/m/c\t0.02s", "FAIL\texample.test/m/b [build failed]", "FAIL\texample.test/m/a\t0.01s", "FAIL"}
	out := runner.Output{Stdout: []byte(strings.Join(lines, "\n"))}
	if got, want := failedPackages(out), []string{"example.test/m/a", "example.test/m/b"}; !slices.Equal(got, want) {
		t.Errorf("failed packages = %q, want %q", got, want)
	}
}
