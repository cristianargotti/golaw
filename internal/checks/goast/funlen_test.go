package goast_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/proof"
)

// funlenLine is one line of expected.txt as golangci-lint prints it.
var funlenLine = regexp.MustCompile(`^(\S+):(\d+):\d+: Function '(\w+)' (is too long|has too many statements) \((\d+) > (\d+)\) \(funlen\)$`)

// expectedFindings turns the golden lines into the keys the check yields.
func expectedFindings(t *testing.T, golden string) []string {
	t.Helper()
	reports := strings.Split(strings.TrimSpace(golden), "\n")
	out := make([]string, 0, len(reports))
	for _, line := range reports {
		m := funlenLine.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("expected.txt line %q is not a funlen report", line)
		}
		message := fmt.Sprintf("function %s is %s lines, the limit is %s: give it one job", m[3], m[5], m[6])
		if m[4] == "has too many statements" {
			message = fmt.Sprintf("function %s has %s statements, the limit is %s: give it one job", m[3], m[5], m[6])
		}
		out = append(out, m[1]+":"+m[2]+" "+message)
	}
	slices.Sort(out)
	return out
}

// corpus reads the Go files of the parity corpus by their bare names.
func corpus(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = string(data)
	}
	return files
}

func TestFunctionLengthMatchesFunlen(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("testdata", "parity")
	files := corpus(t, dir)
	golden, err := os.ReadFile(filepath.Join(dir, "expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := expectedFindings(t, string(golden))
	if len(files) < 6 || len(want) < 6 {
		t.Fatalf("the corpus holds %d files and %d reports, want at least six of each", len(files), len(want))
	}
	res := scan(t, goast.FunctionLength(), files)
	got := make([]string, 0, len(res.Findings))
	for _, f := range res.Findings {
		got = append(got, fmt.Sprintf("%s:%d %s", f.Path, f.Line, f.Message))
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("the check and funlen disagree:\n got %s\nwant %s", strings.Join(got, "\n     "), strings.Join(want, "\n     "))
	}
}

func TestFunctionLengthLeavesAnUnparsableTargetUnmeasured(t *testing.T) {
	t.Parallel()
	broken := scan(t, goast.FunctionLength(), map[string]string{"a.go": "package a\nfunc {"})
	expectUnmeasured(t, broken, "parse a.go")
}

func TestFunctionLengthSkipsTestsAndFollowsTheFloor(t *testing.T) {
	t.Parallel()
	src := "package a\n\nfunc many() {\n\tx := 1\n\tx++\n\tx--\n\t_ = x\n}\n\n" +
		"func long() {\n\tx := add(\n\t\t1,\n\t\t2,\n\t)\n\t_ = x\n}\n\nfunc add(a, b int) int { return a + b }\n\nfunc none()\n"
	c := goast.FunctionLength()
	in := input(c, map[string]string{"a.go": src, "a_test.go": src})
	policy := *proof.Policy()
	policy.Sizes.FunctionLines = 3
	in.Policy = &policy
	res, err := c.Run(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	expectCodes(t, res, "function-statements", "function-length")
	expectLines(t, res, 3, 10)
	if res.Findings[0].Message != "function many has 4 statements, the limit is 3: give it one job" {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}
