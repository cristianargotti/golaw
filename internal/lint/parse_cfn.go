package lint

import (
	"github.com/cristianargotti/golaw/internal/runner"
)

// cfnParseFailure is the rule cfn-lint gives a template it could not parse.
const cfnParseFailure = "E0000"

type cfnMatch struct {
	Filename string `json:"Filename"`
	Level    string `json:"Level"`
	Location struct {
		Start struct {
			LineNumber   int `json:"LineNumber"`
			ColumnNumber int `json:"ColumnNumber"`
		} `json:"Start"`
	} `json:"Location"`
	Message string `json:"Message"`
	Rule    struct {
		ID string `json:"Id"`
	} `json:"Rule"`
}

// cfnAccepts reads cfn-lint's exit status as its bit mask of error 2,
// warning 4 and informational 8; an odd status or a higher bit is its own
// failure.
func cfnAccepts(code int) bool {
	return code >= 0 && code < 16 && code%2 == 0
}

func parseCfnLint(out runner.Output, tool, root string) ([]Issue, error) {
	var matches []cfnMatch
	if err := decodeJSON(out.Stdout, &matches); err != nil {
		return nil, err
	}
	var issues []Issue
	var broken []string
	for _, m := range matches {
		path := relative(root, m.Filename)
		if m.Rule.ID == cfnParseFailure {
			broken = append(broken, path)
			continue
		}
		issues = append(issues, Issue{
			Tool: tool, Rule: m.Rule.ID, Path: path, Line: m.Location.Start.LineNumber,
			Column: m.Location.Start.ColumnNumber, Message: m.Message, Severity: m.Level,
		})
	}
	return issues, unparsed(tool, broken)
}
