package lint

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

type checkovCheck struct {
	CheckID   string  `json:"check_id"`
	CheckName string  `json:"check_name"`
	FilePath  string  `json:"file_path"`
	LineRange []int   `json:"file_line_range"`
	Severity  *string `json:"severity"`
}

type checkovResults struct {
	Failed        []checkovCheck `json:"failed_checks"`
	ParsingErrors []string       `json:"parsing_errors"`
}

// checkovReport is one framework's report; Version alone is the summary a
// run that found no file prints.
type checkovReport struct {
	Results *checkovResults `json:"results"`
	Version string          `json:"checkov_version"`
}

func parseCheckov(out runner.Output, tool, root string) ([]Issue, error) {
	reports, err := checkovReports(out.Stdout)
	if err != nil {
		return nil, err
	}
	var issues []Issue
	var broken []string
	for _, r := range reports {
		found, failed := checkovIssues(r, root)
		issues = append(issues, found...)
		broken = append(broken, failed...)
	}
	for i := range issues {
		issues[i].Tool = tool
	}
	return issues, unparsed(tool, broken)
}

// checkovReports reads one report, a list of them for several frameworks,
// or the summary of a run that found no file.
func checkovReports(data []byte) ([]checkovReport, error) {
	reports, err := decodeCheckov(data)
	if err != nil {
		return nil, err
	}
	for _, r := range reports {
		if r.Results == nil {
			return nil, fmt.Errorf("%w: a checkov report without results", ErrFormat)
		}
	}
	return reports, nil
}

func decodeCheckov(data []byte) ([]checkovReport, error) {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("[")) {
		var reports []checkovReport
		err := decodeJSON(data, &reports)
		return reports, err
	}
	var one checkovReport
	if err := decodeJSON(data, &one); err != nil {
		return nil, err
	}
	if one.Results == nil && one.Version != "" {
		return nil, nil
	}
	return []checkovReport{one}, nil
}

func checkovIssues(r checkovReport, root string) ([]Issue, []string) {
	issues := make([]Issue, 0, len(r.Results.Failed))
	for _, c := range r.Results.Failed {
		issue := Issue{Rule: c.CheckID, Path: checkovPath(root, c.FilePath), Message: c.CheckName}
		if len(c.LineRange) > 0 {
			issue.Line = c.LineRange[0]
		}
		if c.Severity != nil {
			issue.Severity = *c.Severity
		}
		issues = append(issues, issue)
	}
	broken := make([]string, 0, len(r.Results.ParsingErrors))
	for _, p := range r.Results.ParsingErrors {
		broken = append(broken, checkovPath(root, p))
	}
	return issues, broken
}

// checkovPath drops the leading slash of a path outside the root, because
// checkov names paths from the scanned directory that way.
func checkovPath(root, file string) string {
	path := relative(root, file)
	if path == toSlash(file) {
		return strings.TrimPrefix(path, "/")
	}
	return path
}
