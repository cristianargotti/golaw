package newcode

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestParseDiffCountsHunksNotContent(t *testing.T) {
	t.Parallel()
	text := strings.Join([]string{
		"diff --git a/plus.go b/plus.go",
		"new file mode 100644",
		"--- /dev/null",
		"+++ b/plus.go",
		"@@ -0,0 +1,2 @@",
		"+++ x",
		"+@@ -9 +9 @@",
		"diff --git a/a.go b/a.go",
		"--- a/a.go",
		"+++ b/a.go",
		"@@ -4 +4 @@ func A() int {",
		"-\treturn 1",
		"+\treturn 2",
		"@@ -7,0 +8,3 @@",
		"+",
		"+func B() {}",
		"+",
		"\\ No newline at end of file",
		"@@ -12,2 +14,0 @@",
		"-x",
		"-y",
		"diff --git a/gone.go b/gone.go",
		"--- a/gone.go",
		"+++ /dev/null",
		"@@ -1 +0,0 @@",
		"-package gone",
		"",
	}, "\n")
	got, err := parseDiff(text)
	want := map[string][]int{"plus.go": {1, 2}, "a.go": {4, 8, 9, 10}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDiff = %v, %v, want %v", got, err, want)
	}
	if empty, err := parseDiff(""); err != nil || len(empty) != 0 {
		t.Fatalf("an empty diff = %v, %v", empty, err)
	}
}

func TestParseDiffRefusesWhatGitNeverWrites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		text string
	}{
		{"red-a-short-header", "+++ b/a.go\n@@ broken\n"},
		{"red-a-bad-old-start", "+++ b/a.go\n@@ -x +1 @@\n"},
		{"red-a-bad-new-count", "+++ b/a.go\n@@ -1 +1,y @@\n"},
		{"red-a-negative-count", "+++ b/a.go\n@@ -1 +1,-2 @@\n"},
		{"red-a-header-without-signs", "+++ b/a.go\n@@ 1 1 @@\n"},
		{"red-a-bad-quote", "+++ \"b/bad\\q.go\"\n"},
		{"red-a-path-without-the-prefix", "+++ c/a.go\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got, err := parseDiff(tc.text); !errors.Is(err, ErrDiff) || got != nil {
				t.Fatalf("parseDiff = %v, %v, want ErrDiff", got, err)
			}
		})
	}
}

func TestPathOfReadsEveryFormGitWrites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		line string
		want string
	}{
		{"+++ b/a.go", "a.go"},
		{"+++ b/pkg/ação x.go\t", "pkg/ação x.go"},
		{"+++ \"b/pkg/a\\tb.go\"", "pkg/a\tb.go"},
		{"+++ \"b/pkg/\\303\\247.go\"", "pkg/ç.go"},
		{"+++ /dev/null", ""},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got, err := pathOf(tc.line); err != nil || got != tc.want {
				t.Fatalf("pathOf(%q) = %q, %v, want %q", tc.line, got, err, tc.want)
			}
		})
	}
}

func TestBoundedRefusesADiffTheRunnerCut(t *testing.T) {
	t.Parallel()
	if err := bounded(runner.Output{Stdout: make([]byte, runner.MaxOutputBytes)}); !errors.Is(err, ErrDiff) {
		t.Fatalf("a cut diff = %v, want ErrDiff", err)
	}
	if err := bounded(runner.Output{Stdout: make([]byte, runner.MaxOutputBytes-1)}); err != nil {
		t.Fatalf("a diff under the bound = %v", err)
	}
}
