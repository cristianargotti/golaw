package lint_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/runner"
)

// ruff 0.17 prints the format check as diagnostics; the kit asks for the
// concise form, one line per file, recorded here from the pinned tool.
func TestParseReadsTheConciseFormatCheck(t *testing.T) {
	t.Parallel()
	runParseCases(t, map[string]parseCase{
		"a file to reformat beside a file ruff cannot parse": {lint.FormatRuffFormat, "ruff", runner.Output{Stdout: recorded(t, lint.FormatRuffFormat, "concise.txt"), Code: 2}, []lint.Issue{
			{Tool: "ruff", Rule: "format", Path: "deploy/report.py", Line: 3, Column: 11, Message: "the file is not formatted as ruff format formats it", Severity: "error"},
		}, []string{"deploy/bad.py"}},
		"a file to reformat on a Windows path": {lint.FormatRuffFormat, "ruff", runner.Output{Stdout: []byte(`C:\work\repo\deploy\report.py:3:11: unformatted: File would be reformatted` + "\n1 file would be reformatted\n"), Code: 1}, []lint.Issue{
			{Tool: "ruff", Rule: "format", Path: "C:/work/repo/deploy/report.py", Line: 3, Column: 11, Message: "the file is not formatted as ruff format formats it", Severity: "error"},
		}, nil},
		"a clean run prints the summary alone (near miss)": {lint.FormatRuffFormat, "ruff", runner.Output{Stdout: []byte("1 file already formatted\n")}, nil, nil},
	})
}

func TestParseRefusesAConciseLineItCannotRead(t *testing.T) {
	t.Parallel()
	for name, line := range map[string]string{
		"another code":         "/work/repo/deploy/report.py:3:11: E501 Line too long",
		"a row past int range": "/work/repo/deploy/report.py:99999999999999999999:11: unformatted: File would be reformatted",
		"a column past range":  "/work/repo/deploy/report.py:3:99999999999999999999: unformatted: File would be reformatted",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out := runner.Output{Stdout: []byte(line + "\n"), Code: 1}
			if got, err := lint.Parse(lint.FormatRuffFormat, "ruff", out, root); !errors.Is(err, lint.ErrFormat) || got != nil {
				t.Errorf("Parse = %+v, %v, want ErrFormat and no issue", got, err)
			}
		})
	}
}
