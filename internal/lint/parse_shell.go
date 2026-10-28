package lint

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

const shfmtMessage = "the file is not formatted as shfmt -i 2 -ci formats it"

var (
	shfmtHunk  = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
	shfmtError = regexp.MustCompile(`^(.+?):\d+:\d+: .+$`)
)

type shellcheckComment struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Level   string `json:"level"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// shellcheckReport points at its list so a report without the key reads
// as a format golaw does not know rather than as a clean run.
type shellcheckReport struct {
	Comments *[]shellcheckComment `json:"comments"`
}

// parseShellcheck keeps the parse failures (SC1xxx) as issues, because
// shellcheck reports them on their line and every shellcheck finding is
// feedback (M6.6.3).
func parseShellcheck(out runner.Output, tool, root string) ([]Issue, error) {
	var report shellcheckReport
	if err := decodeJSON(out.Stdout, &report); err != nil {
		return nil, err
	}
	if report.Comments == nil {
		return nil, fmt.Errorf("%w: %s printed no comments", ErrFormat, tool)
	}
	issues := make([]Issue, 0, len(*report.Comments))
	for _, c := range *report.Comments {
		issues = append(issues, Issue{
			Tool: tool, Rule: "SC" + strconv.Itoa(c.Code), Path: relative(root, c.File),
			Line: c.Line, Column: c.Column, Message: c.Message, Severity: c.Level,
		})
	}
	return issues, nil
}

func parseShfmt(out runner.Output, tool, root string) ([]Issue, error) {
	issues, err := shfmtDiff(out.Stdout, tool, root)
	if err != nil {
		return nil, err
	}
	broken := matchedPaths(out.Stderr, shfmtError, root)
	if out.Code == 1 && len(issues) == 0 && len(broken) == 0 {
		return nil, fmt.Errorf("%w: %s exited 1 with no diff and no error line", ErrFormat, tool)
	}
	return issues, unparsed(tool, broken)
}

// shfmtDiff answers one issue per file of a unified diff, at the new side
// start of its first hunk; output that holds no header pair is ErrFormat.
func shfmtDiff(data []byte, tool, root string) ([]Issue, error) {
	var issues []Issue
	previous, open := "", false
	lines := bufio.NewScanner(bytes.NewReader(data))
	for lines.Scan() {
		line := lines.Text()
		if path, ok := strings.CutPrefix(line, "+++ "); ok && strings.HasPrefix(previous, "--- ") {
			path, _, _ = strings.Cut(path, "\t")
			issues = append(issues, Issue{
				Tool: tool, Rule: "format", Path: relative(root, path), Line: 1, Column: 1,
				Message: shfmtMessage, Severity: severityError,
			})
			open = true
		} else if m := shfmtHunk.FindStringSubmatch(line); m != nil && open {
			start, err := number(m[1])
			if err != nil {
				return nil, err
			}
			issues[len(issues)-1].Line, open = start, false
		}
		previous = line
	}
	if len(issues) == 0 && len(bytes.TrimSpace(data)) > 0 {
		return nil, fmt.Errorf("%w: %s printed no diff header", ErrFormat, tool)
	}
	return issues, nil
}
