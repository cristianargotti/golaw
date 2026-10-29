package lint

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

const ruffFormatMessage = "the file is not formatted as ruff format formats it"

var (
	ruffSummary    = regexp.MustCompile(`^\d+ files? (would be reformatted|already formatted)(, \d+ files? already formatted)?$`)
	ruffParseError = regexp.MustCompile(`^error: Failed to parse (.+?):\d+:\d+: `)
	// ruffConcise is the line ruff 0.17 prints per file under the concise
	// output form; any other code on such a line is a format golaw does not
	// know, never a clean file.
	ruffConcise = regexp.MustCompile(`^(.+?):(\d+):(\d+): (unformatted|invalid-syntax): `)
)

type ruffLocation struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

// ruffDiagnostic has a nil code for a file ruff could not read.
type ruffDiagnostic struct {
	Code     *string      `json:"code"`
	Filename string       `json:"filename"`
	Message  string       `json:"message"`
	Location ruffLocation `json:"location"`
}

// parseRuff gives every issue the severity error, because ruff reports no
// level of its own.
func parseRuff(out runner.Output, tool, root string) ([]Issue, error) {
	var diagnostics []ruffDiagnostic
	if err := decodeJSON(out.Stdout, &diagnostics); err != nil {
		return nil, err
	}
	var issues []Issue
	var broken []string
	for _, d := range diagnostics {
		path := relative(root, d.Filename)
		if d.Code == nil || *d.Code == "invalid-syntax" {
			broken = append(broken, path)
			continue
		}
		issues = append(issues, Issue{
			Tool: tool, Rule: *d.Code, Path: path, Line: d.Location.Row, Column: d.Location.Column,
			Message: d.Message, Severity: severityError,
		})
	}
	return issues, unparsed(tool, broken)
}

// parseRuffFormat reads both forms of the check: the Would reformat lines
// of the older default and the concise diagnostics of ruff 0.17.
func parseRuffFormat(out runner.Output, tool, root string) ([]Issue, error) {
	var issues []Issue
	var broken []string
	lines := bufio.NewScanner(bytes.NewReader(out.Stdout))
	for lines.Scan() {
		line := string(bytes.TrimSpace(lines.Bytes()))
		if path, ok := strings.CutPrefix(line, "Would reformat: "); ok && path != "" {
			issues = append(issues, ruffFormatIssue(tool, relative(root, path), 1, 1))
			continue
		}
		if m := ruffConcise.FindStringSubmatch(line); m != nil {
			issue, err := ruffConciseIssue(m, tool, root)
			switch {
			case err != nil:
				return nil, err
			case m[4] == "invalid-syntax":
				broken = append(broken, issue.Path)
			default:
				issues = append(issues, issue)
			}
			continue
		}
		if line != "" && !ruffSummary.MatchString(line) {
			return nil, fmt.Errorf("%w: %s printed %q", ErrFormat, tool, line)
		}
	}
	broken = append(broken, matchedPaths(out.Stderr, ruffParseError, root)...)
	if out.Code == 2 && len(broken) == 0 {
		return nil, toolFailed(tool, out)
	}
	return issues, unparsed(tool, broken)
}

func ruffFormatIssue(tool, path string, line, column int) Issue {
	return Issue{
		Tool: tool, Rule: "format", Path: path, Line: line, Column: column,
		Message: ruffFormatMessage, Severity: severityError,
	}
}

// ruffConciseIssue reads the groups of a ruffConcise match: path, row and
// column.
func ruffConciseIssue(m []string, tool, root string) (Issue, error) {
	line, err := number(m[2])
	if err != nil {
		return Issue{}, err
	}
	column, err := number(m[3])
	if err != nil {
		return Issue{}, err
	}
	return ruffFormatIssue(tool, relative(root, m[1]), line, column), nil
}

// matchedPaths answers the first group of every line of data the pattern
// matches, each relative to root.
func matchedPaths(data []byte, pattern *regexp.Regexp, root string) []string {
	var paths []string
	lines := bufio.NewScanner(bytes.NewReader(data))
	for lines.Scan() {
		if m := pattern.FindStringSubmatch(string(bytes.TrimSpace(lines.Bytes()))); m != nil {
			paths = append(paths, relative(root, m[1]))
		}
	}
	return paths
}
