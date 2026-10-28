package lint

import (
	"github.com/cristianargotti/golaw/internal/runner"
)

// eslintMessage is one message of an ESLint file entry; a nil rule is a
// message of ESLint itself, such as a parsing error.
type eslintMessage struct {
	RuleID   *string `json:"ruleId"`
	Fatal    bool    `json:"fatal"`
	Severity int     `json:"severity"`
	Message  string  `json:"message"`
	Line     int     `json:"line"`
	Column   int     `json:"column"`
}

type eslintFile struct {
	FilePath   string          `json:"filePath"`
	Messages   []eslintMessage `json:"messages"`
	Suppressed []eslintMessage `json:"suppressedMessages"`
}

func parseESLint(out runner.Output, tool, root string) ([]Issue, error) {
	var files []eslintFile
	if err := decodeJSON(out.Stdout, &files); err != nil {
		return nil, err
	}
	var issues []Issue
	var broken []string
	for _, f := range files {
		path := relative(root, f.FilePath)
		for _, m := range f.Messages {
			if m.Fatal || m.RuleID == nil {
				broken = append(broken, path)
				continue
			}
			issues = append(issues, eslintIssue(tool, path, m, false))
		}
		for _, m := range f.Suppressed {
			if m.RuleID != nil {
				issues = append(issues, eslintIssue(tool, path, m, true))
			}
		}
	}
	return issues, unparsed(tool, broken)
}

func eslintIssue(tool, path string, m eslintMessage, suppressed bool) Issue {
	severity := "warning"
	if m.Severity == 2 {
		severity = severityError
	}
	return Issue{
		Tool: tool, Rule: *m.RuleID, Path: path, Line: m.Line, Column: m.Column,
		Message: m.Message, Severity: severity, Suppressed: suppressed,
	}
}

type stylelintWarning struct {
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Text     string `json:"text"`
}

type stylelintFile struct {
	Source      string             `json:"source"`
	Warnings    []stylelintWarning `json:"warnings"`
	ParseErrors []any              `json:"parseErrors"`
}

// parseStylelint reads stderr when stdout is empty, because stylelint 16
// and later print the report there.
func parseStylelint(out runner.Output, tool, root string) ([]Issue, error) {
	data := out.Stdout
	if len(data) == 0 {
		data = out.Stderr
	}
	var files []stylelintFile
	if err := decodeJSON(data, &files); err != nil {
		return nil, err
	}
	var issues []Issue
	var broken []string
	for _, f := range files {
		path := relative(root, f.Source)
		if len(f.ParseErrors) > 0 {
			broken = append(broken, path)
		}
		for _, w := range f.Warnings {
			if w.Rule == "CssSyntaxError" {
				broken = append(broken, path)
				continue
			}
			issues = append(issues, Issue{
				Tool: tool, Rule: w.Rule, Path: path, Line: w.Line, Column: w.Column,
				Message: w.Text, Severity: w.Severity,
			})
		}
	}
	return issues, unparsed(tool, broken)
}
