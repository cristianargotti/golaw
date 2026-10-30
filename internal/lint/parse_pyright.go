package lint

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/runner"
)

type pyrightPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type pyrightDiagnostic struct {
	File     string `json:"file"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Range    struct {
		Start pyrightPosition `json:"start"`
	} `json:"range"`
	Rule string `json:"rule"`
}

// pyrightReport points at its list so a report without the key reads as a
// format golaw does not know rather than as a clean run.
type pyrightReport struct {
	Diagnostics *[]pyrightDiagnostic `json:"generalDiagnostics"`
}

func parsePyright(out runner.Output, tool, root string) ([]Issue, error) {
	var report pyrightReport
	if err := decodeJSON(out.Stdout, &report); err != nil {
		return nil, err
	}
	if report.Diagnostics == nil {
		return nil, fmt.Errorf("%w: %s printed no generalDiagnostics", ErrFormat, tool)
	}
	var issues []Issue
	var broken []string
	for _, d := range *report.Diagnostics {
		path := relative(root, d.File)
		switch {
		case d.Severity != severityError && d.Severity != "warning":
			continue
		case d.Severity == severityError && d.Rule == "":
			broken = append(broken, path)
			continue
		}
		// pyright counts lines and characters from zero.
		issues = append(issues, Issue{
			Tool: tool, Rule: d.Rule, Path: path, Line: d.Range.Start.Line + 1,
			Column: d.Range.Start.Character + 1, Message: d.Message, Severity: d.Severity,
		})
	}
	return issues, unparsed(tool, broken)
}
