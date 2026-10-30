package lint_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/runner"
)

const root = "/work/repo"

func recorded(t *testing.T, format lint.Format, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "output", string(format), name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// parseCase is one recorded output and what its parser must answer.
type parseCase struct {
	format   lint.Format
	tool     string
	out      runner.Output
	want     []lint.Issue
	unparsed []string // the paths ErrUnparsed must name, none for a nil error
}

func runParseCases(t *testing.T, cases map[string]parseCase) {
	t.Helper()
	for name := range cases {
		tc := cases[name]
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkParse(t, &tc)
		})
	}
}

func checkParse(t *testing.T, tc *parseCase) {
	t.Helper()
	got, err := lint.Parse(tc.format, tc.tool, tc.out, root)
	if !slices.Equal(got, tc.want) {
		t.Errorf("issues = %+v\nwant %+v", got, tc.want)
	}
	switch {
	case len(tc.unparsed) == 0 && err != nil:
		t.Errorf("Parse = %v, want no error", err)
	case len(tc.unparsed) > 0 && !errors.Is(err, lint.ErrUnparsed):
		t.Errorf("Parse = %v, want ErrUnparsed", err)
	}
	for _, path := range tc.unparsed {
		if err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("Parse = %v, want it to name %s", err, path)
		}
	}
}

func TestParseReadsTheWebAndPythonFormats(t *testing.T) {
	t.Parallel()
	stylelint := recorded(t, lint.FormatStylelint, "findings.json")
	runParseCases(t, map[string]parseCase{
		"eslint findings": {lint.FormatESLint, "eslint", runner.Output{Stdout: recorded(t, lint.FormatESLint, "findings.json"), Code: 1}, []lint.Issue{
			{Tool: "eslint", Rule: "sonarjs/no-unenclosed-multiline-block", Path: "views/panel.js", Line: 12, Column: 5, Message: "This line will not be executed conditionally; only the first line of this 2-line block will be. The rest will execute unconditionally.", Severity: "error"},
			{Tool: "eslint", Rule: "sonarjs/no-nested-conditional", Path: "views/panel.js", Line: 20, Column: 14, Message: "Extract this nested ternary operation into an independent statement.", Severity: "warning"},
			{Tool: "eslint", Rule: "sonarjs/no-nested-conditional", Path: "views/panel.js", Line: 31, Column: 9, Message: "Extract this nested ternary operation into an independent statement.", Severity: "error", Suppressed: true},
		}, nil},
		"eslint clean": {lint.FormatESLint, "eslint", runner.Output{Stdout: []byte("[]\n")}, nil, nil},
		"eslint parse error": {lint.FormatESLint, "eslint", runner.Output{Stdout: recorded(t, lint.FormatESLint, "fatal.json"), Code: 1}, []lint.Issue{
			{Tool: "eslint", Rule: "no-undef", Path: "views/common.js", Line: 3, Column: 1, Message: "'render' is not defined.", Severity: "error"},
		}, []string{"views/panel.html"}},
		"stylelint on stdout": {lint.FormatStylelint, "stylelint", runner.Output{Stdout: stylelint, Code: 2}, []lint.Issue{
			{Tool: "stylelint", Rule: "no-duplicate-selectors", Path: "views/panel.css", Line: 7, Column: 1, Message: "Unexpected duplicate selector \"table\", first used at line 3 (no-duplicate-selectors)", Severity: "error"},
		}, []string{"views/broken.css"}},
		"stylelint on stderr": {lint.FormatStylelint, "stylelint", runner.Output{Stderr: stylelint, Code: 2}, []lint.Issue{
			{Tool: "stylelint", Rule: "no-duplicate-selectors", Path: "views/panel.css", Line: 7, Column: 1, Message: "Unexpected duplicate selector \"table\", first used at line 3 (no-duplicate-selectors)", Severity: "error"},
		}, []string{"views/broken.css"}},
		"ruff findings": {lint.FormatRuff, "ruff", runner.Output{Stdout: recorded(t, lint.FormatRuff, "findings.json"), Code: 1}, []lint.Issue{
			{Tool: "ruff", Rule: "S307", Path: "test/deploy/test_install.py", Line: 25, Column: 12, Message: "Use of possibly insecure function; consider using `ast.literal_eval`", Severity: "error"},
		}, []string{"scripts/broken.py"}},
		"ruff clean": {lint.FormatRuff, "ruff", runner.Output{Stdout: []byte("[]")}, nil, nil},
		"ruff format check": {lint.FormatRuffFormat, "ruff", runner.Output{Stdout: recorded(t, lint.FormatRuffFormat, "findings.txt"), Code: 1}, []lint.Issue{
			{Tool: "ruff", Rule: "format", Path: "scripts/metricas.py", Line: 1, Column: 1, Message: "the file is not formatted as ruff format formats it", Severity: "error"},
			{Tool: "ruff", Rule: "format", Path: "deploy/install.py", Line: 1, Column: 1, Message: "the file is not formatted as ruff format formats it", Severity: "error"},
		}, nil},
		"ruff format parse error": {lint.FormatRuffFormat, "ruff", runner.Output{Stdout: []byte("1 file already formatted\n"), Stderr: []byte("error: Failed to parse scripts/broken.py:1:7: Expected a parameter\n"), Code: 2}, nil, []string{"scripts/broken.py"}},
		"ruff format clean":       {lint.FormatRuffFormat, "ruff", runner.Output{Stdout: []byte("4 files already formatted\n")}, nil, nil},
		"pyright findings": {lint.FormatPyright, "pyright", runner.Output{Stdout: recorded(t, lint.FormatPyright, "findings.json"), Code: 1}, []lint.Issue{
			{Tool: "pyright", Rule: "reportPossiblyUnbound", Path: "deploy/install.py", Line: 12, Column: 5, Message: "\"session\" is possibly unbound", Severity: "error"},
		}, []string{"scripts/broken.py"}},
	})
}

func TestParseNeverAnswersCleanForWhatItCannotRead(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		format lint.Format
		out    runner.Output
		want   error
	}{
		"unknown format":              {"sarif", runner.Output{Stdout: []byte("{}")}, lint.ErrFormat},
		"html instead of json":        {lint.FormatESLint, runner.Output{Stdout: []byte("<html>"), Code: 1}, lint.ErrFormat},
		"nothing printed":             {lint.FormatRuff, runner.Output{}, lint.ErrFormat},
		"an object where a list goes": {lint.FormatStylelint, runner.Output{Stdout: []byte(`{"warnings": []}`), Code: 2}, lint.ErrFormat},
		"a line the check form lacks": {lint.FormatRuffFormat, runner.Output{Stdout: []byte("reformatted everything\n"), Code: 1}, lint.ErrFormat},
		"eslint crashed":              {lint.FormatESLint, runner.Output{Stderr: []byte("Oops! Something went wrong! :(\n"), Code: 2}, lint.ErrToolFailed},
		"pyright configuration error": {lint.FormatPyright, runner.Output{Stderr: []byte("Config file could not be parsed\n"), Code: 3}, lint.ErrToolFailed},
		"stylelint configuration":     {lint.FormatStylelint, runner.Output{Stderr: []byte("No configuration provided\n"), Code: 78}, lint.ErrToolFailed},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := lint.Parse(tc.format, "tool", tc.out, root)
			if !errors.Is(err, tc.want) || got != nil {
				t.Errorf("Parse = %+v, %v, want %v and no issue", got, err, tc.want)
			}
		})
	}
}

func TestParseNamesAWindowsPathRelativeToItsRoot(t *testing.T) {
	t.Parallel()
	out := runner.Output{Stdout: []byte(`[{"filePath":"C:\\work\\repo\\views\\a.js","messages":[{"ruleId":"no-undef","severity":2,"message":"x","line":1,"column":2}]}]`), Code: 1}
	got, err := lint.Parse(lint.FormatESLint, "eslint", out, `C:\work\repo`)
	if err != nil || len(got) != 1 || got[0].Path != "views/a.js" {
		t.Errorf("Parse = %+v, %v, want views/a.js", got, err)
	}
	outside, err := lint.Parse(lint.FormatESLint, "eslint", out, `D:\other`)
	if err != nil || len(outside) != 1 || outside[0].Path != "C:/work/repo/views/a.js" {
		t.Errorf("a path outside the root = %+v, %v, want it kept in slash form", outside, err)
	}
}

func TestCodeIsTheKebabFormOfARule(t *testing.T) {
	t.Parallel()
	for rule, want := range map[string]string{
		"sonarjs/no-unenclosed-multiline-block": "sonarjs-no-unenclosed-multiline-block",
		"S105":                                  "s105",
		"DL3002":                                "dl3002",
		"SC2086":                                "sc2086",
		"CKV_AWS_18":                            "ckv-aws-18",
		"reportPossiblyUnbound":                 "reportpossiblyunbound",
		"@html-eslint/require-lang":             "html-eslint-require-lang",
		"--":                                    "",
		"":                                      "",
	} {
		if got := lint.Code(rule); got != want {
			t.Errorf("Code(%q) = %q, want %q", rule, got, want)
		}
	}
}
