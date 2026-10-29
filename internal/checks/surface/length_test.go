package surface_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// lines is n lines of the text, each ended by a line break.
func lines(n int, text string) string { return strings.Repeat(text+"\n", n) }

func TestFileLengthJudgesTheSourceLanguages(t *testing.T) {
	t.Parallel()
	runCases(t, surface.FileLength(), []fileCase{
		{"a shell script of 301 lines", "scripts/smoke.sh", "#!/bin/sh\n" + lines(300, "echo step"), []string{"scripts/smoke.sh:0 file-length"}},
		{"a python file", "tools/report.py", lines(301, "x = 1"), []string{"tools/report.py:0 file-length"}},
		{"a module", "views/common.mjs", lines(301, "run();"), []string{"views/common.mjs:0 file-length"}},
		{"a typescript file", "src/a.ts", lines(301, "run();"), []string{"src/a.ts:0 file-length"}},
		{"a jsx file", "src/a.jsx", lines(301, "run();"), []string{"src/a.jsx:0 file-length"}},
		{"a tsx file", "src/a.tsx", lines(301, "run();"), []string{"src/a.tsx:0 file-length"}},
		{"no final line break counts the last line", "a.js", lines(300, "run();") + "run();", []string{"a.js:0 file-length"}},
		{"a bash script without extension", "bin/up", "#!/bin/bash\n" + lines(300, "up"), []string{"bin/up:0 file-length"}},
	})
}

func TestFileLengthLeavesMarkupDataAndBundlesAlone(t *testing.T) {
	t.Parallel()
	runCases(t, surface.FileLength(), []fileCase{
		{"exactly 300 lines", "scripts/smoke.sh", "#!/bin/sh\n" + lines(299, "echo step"), nil},
		{"an html view", "views/report.html", lines(1428, "<p>row</p>"), nil},
		{"a style sheet", "views/app.css", lines(400, "a { color: red; }"), nil},
		{"a yaml document", "api/openapi.yaml", "openapi: 3.1.0\n" + lines(400, "# path"), nil},
		{"a cloudformation template", "infra/20-app.yml", "AWSTemplateFormatVersion: \"2010-09-09\"\nResources: {}\n" + lines(400, "# note"), nil},
		{"a json document", "data/a.json", "[\n" + lines(400, "1,") + "1\n]\n", nil},
		{"a minified bundle", "views/charts.min.js", lines(400, "x"), nil},
		{"a generated mock", "test/mocks/a.ts", lines(400, "x"), nil},
		{"a declaration file", "types/index.d.ts", lines(400, "declare const x: number;"), nil},
		{"a fixture", "pkg/testdata/a.sh", lines(400, "x"), nil},
		{"a go file", "a.go", "package a\n" + lines(400, "// x"), nil},
	})
}

func TestFileLengthNamesTheCountAndFollowsThePolicy(t *testing.T) {
	t.Parallel()
	c := surface.FileLength()
	res := scanFiles(t, c, map[string]string{"scripts/smoke.sh": lines(312, "echo")})
	want := []model.Finding{{Path: "scripts/smoke.sh", Code: "file-length", Message: "has 312 lines, the limit is 300: split it by responsibility.", Severity: model.SeverityDeny}}
	if !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("findings = %+v, want %+v", res.Findings, want)
	}
	in := scanInput(c, map[string]string{"a.py": "x = 1\ny = 2\nz = 3\n"})
	policy := *proof.Policy()
	policy.Sizes.FileLines = 2
	in.Policy = &policy
	expectFound(t, run(t, c, in), "a.py:0 file-length")
}

func TestFileLengthDeniesOnlyAFileThatCrossesTheLimit(t *testing.T) {
	t.Parallel()
	c := surface.FileLength()
	legacy := map[string]string{"a.py": lines(310, "x = 1")}
	expectFound(t, run(t, c, writeInput(c, legacy, map[string]string{"a.py": lines(305, "x = 1")})))
	expectFound(t, run(t, c, writeInput(c, legacy, map[string]string{"a.py": lines(320, "x = 1")})))
	within := map[string]string{"a.py": lines(300, "x = 1")}
	expectFound(t, run(t, c, writeInput(c, within, map[string]string{"a.py": lines(301, "x = 1")})), "a.py:0 file-length")
}
