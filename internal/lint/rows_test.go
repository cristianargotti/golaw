package lint_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/lint"
)

const rowsYAML = `- {sonar: "javascript:S2681", tool: eslint, rule: sonarjs/no-unenclosed-multiline-block, where: [L], match: exact, set: parity}
- {sonar: "python:S1523", tool: ruff, rule: S307, where: [H, L], match: close, set: parity}
- {sonar: "docker:S6471", tool: hadolint, rule: DL3002, where: [H, L], match: close, set: strict}
`

func TestDecodeRowsReadsTheRuleMap(t *testing.T) {
	t.Parallel()
	rows, err := lint.DecodeRows([]byte(rowsYAML))
	if err != nil || len(rows) != 3 {
		t.Fatalf("DecodeRows = %+v, %v", rows, err)
	}
	want := lint.Row{Sonar: "python:S1523", Tool: "ruff", Rule: "S307", Where: []string{"H", "L"}, Match: "close", Set: "parity"}
	if got := rows[1]; got.Sonar != want.Sonar || got.Tool != want.Tool || got.Rule != want.Rule || !slices.Equal(got.Where, want.Where) || got.Match != want.Match || got.Set != want.Set {
		t.Errorf("row 1 = %+v, want %+v", got, want)
	}
}

func TestDecodeRowsRefusesARowTheMapCannotTrust(t *testing.T) {
	t.Parallel()
	tests := map[string]struct{ old, new string }{
		"unknown key":              {"match: exact,", "match: exact, owner: x,"},
		"missing set":              {", set: strict}", "}"},
		"unknown place":            {"where: [L]", "where: [X]"},
		"repeated place":           {"where: [H, L], match: close, set: parity", "where: [H, H], match: close, set: parity"},
		"no place":                 {"where: [L]", "where: []"},
		"unknown match":            {"match: exact", "match: fuzzy"},
		"unknown set":              {"set: strict", "set: lenient"},
		"sonar key without a rule": {`"docker:S6471"`, `"docker"`},
		"empty rule":               {"rule: DL3002", `rule: ""`},
		"the same tool rule twice": {"tool: hadolint, rule: DL3002", "tool: ruff, rule: S307"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			changed := strings.Replace(rowsYAML, tc.old, tc.new, 1)
			if changed == rowsYAML {
				t.Fatal("the mutation did not change the map")
			}
			if _, err := lint.DecodeRows([]byte(changed)); !errors.Is(err, lint.ErrRows) {
				t.Errorf("DecodeRows = %v, want ErrRows", err)
			}
		})
	}
	if _, err := lint.DecodeRows([]byte("rows:\n  - {sonar: \"python:S1523\", tool: ruff, rule: S307, where: [L], match: close, set: parity}\n")); !errors.Is(err, lint.ErrRows) {
		t.Errorf("a mapping at the top = %v, want ErrRows: a rule map is a sequence of rows", err)
	}
}
