package goast_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/proof"
)

const exactFile = "package a\n\n" + "// line\n"

func TestFileLengthBoundary(t *testing.T) {
	t.Parallel()
	exact := "package a\n\n" + strings.Repeat("// line\n", 298)
	res := scan(t, goast.FileLength(), map[string]string{
		"a.go":  exact,
		"b.go":  exact + "// one more\n",
		"c.go":  exact + "// no newline at the end",
		"empty": "",
	})
	expectCodes(t, res, "file-length", "file-length")
	expectLines(t, res, 0, 0)
	want := "file has 301 lines, the limit is 300: split it by responsibility"
	if res.Findings[0].Path != "b.go" || res.Findings[1].Path != "c.go" || res.Findings[0].Message != want {
		t.Errorf("findings = %+v", res.Findings)
	}
}

func TestFileLengthFollowsThePolicyFloor(t *testing.T) {
	t.Parallel()
	c := goast.FileLength()
	in := input(c, map[string]string{"a.go": exactFile, "b.go": "package b\n"})
	policy := *proof.Policy()
	policy.Sizes.FileLines = 2
	in.Policy = &policy
	res, err := c.Run(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	expectCodes(t, res, "file-length")
}
