package lint_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/lint"
)

func TestRowsJoinThePartsInOrder(t *testing.T) {
	t.Parallel()
	rows := lint.Rows()
	if len(rows) == 0 {
		t.Fatal("no rule map row")
	}
	keys := make([]string, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, r.Sonar+"\x00"+r.Tool+"\x00"+r.Rule)
	}
	if !slices.IsSorted(keys) {
		t.Error("rows are not sorted by sonar, tool and rule")
	}
	first := rows[0]
	if got, ok := lint.SonarKey(first.Tool, first.Rule); !ok || got != first.Sonar {
		t.Errorf("SonarKey(%s, %s) = %q, %t, want %q", first.Tool, first.Rule, got, ok, first.Sonar)
	}
	if _, ok := lint.SonarKey("ruff", "NOT-A-RULE"); ok {
		t.Error("an unmapped rule has a Sonar key")
	}
	if !strings.Contains(first.Sonar, ":S") {
		t.Errorf("first row %+v", first)
	}
}
