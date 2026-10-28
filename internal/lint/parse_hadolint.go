package lint

import (
	"github.com/cristianargotti/golaw/internal/runner"
)

// hadolintParseFailure is the code hadolint gives a Dockerfile it could not
// parse.
const hadolintParseFailure = "DL1000"

type hadolintComment struct {
	Code    string `json:"code"`
	File    string `json:"file"`
	Level   string `json:"level"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

// parseHadolint leaves the path empty for the stdin forms the hook uses, so
// its caller fills in the path it wrote.
func parseHadolint(out runner.Output, tool, root string) ([]Issue, error) {
	var comments []hadolintComment
	if err := decodeJSON(out.Stdout, &comments); err != nil {
		return nil, err
	}
	var issues []Issue
	var broken []string
	for _, c := range comments {
		path := relative(root, c.File)
		if c.Code == hadolintParseFailure {
			broken = append(broken, stdinName(path))
			continue
		}
		issues = append(issues, Issue{
			Tool: tool, Rule: c.Code, Path: path, Line: c.Line, Column: c.Column,
			Message: c.Message, Severity: c.Level,
		})
	}
	return issues, unparsed(tool, broken)
}

// stdinName names an input that has no path of its own in an error.
func stdinName(path string) string {
	if path == "" {
		return "stdin"
	}
	return path
}
