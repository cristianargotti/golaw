package lint

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"

	"github.com/cristianargotti/golaw/internal/runner"
)

var yamllintLine = regexp.MustCompile(`^(.+):(\d+):(\d+): \[(error|warning)\] (.+) \(([a-z-]+)\)$`)

// parseYamllint reads the parsable form; the rule syntax is yamllint's word
// for a file it could not parse.
func parseYamllint(out runner.Output, tool, root string) ([]Issue, error) {
	var issues []Issue
	var broken []string
	lines := bufio.NewScanner(bytes.NewReader(out.Stdout))
	for lines.Scan() {
		line := string(bytes.TrimSpace(lines.Bytes()))
		if line == "" {
			continue
		}
		m := yamllintLine.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("%w: %s printed %q", ErrFormat, tool, line)
		}
		path := relative(root, m[1])
		if m[6] == "syntax" {
			broken = append(broken, path)
			continue
		}
		row, err := number(m[2])
		if err != nil {
			return nil, err
		}
		column, err := number(m[3])
		if err != nil {
			return nil, err
		}
		issues = append(issues, Issue{
			Tool: tool, Rule: m[6], Path: path, Line: row, Column: column,
			Message: m[5], Severity: m[4],
		})
	}
	return issues, unparsed(tool, broken)
}

// number reads a position a pattern matched as digits; only a value past
// the int range fails, and that output is not one golaw knows.
func number(digits string) (int, error) {
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrFormat, err)
	}
	return n, nil
}
