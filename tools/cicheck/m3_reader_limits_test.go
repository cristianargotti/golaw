package cicheck_test

import (
	"strings"
	"testing"
)

// TestM3ReaderRowsNameEachRewrite holds the reader table to the limits a
// check states in its reason, each beside the rewrite that clears it, so a
// reason that points to docs/rules.md finds its row there.
func TestM3ReaderRowsNameEachRewrite(t *testing.T) {
	t.Parallel()
	rows := []struct{ reader, limit, resolve string }{
		{"readMakefile", "an opaque recipe", "write the delegation as a prerequisite of ci or as make <target> on its own line"},
		{"readMakefile", "make or gmake as a word anywhere on the line, a quoted argument included", "add the missing target to ci"},
		{"readMakefile", "named as double-colon rule <target>::, only for a check that needs one of them", "use a single-colon rule"},
		{"readLefthook", "ignores such a file and every construct outside the subset", "one run string in lefthook.yml itself"},
	}
	lines := strings.Split(string(kitBytes(t, m3RulesPath)), "\n")
	for _, row := range rows {
		found := false
		for _, line := range lines {
			found = found || strings.HasPrefix(line, "| "+row.reader+" | ") && strings.Contains(line, row.limit) && strings.Contains(line, row.resolve)
		}
		if !found {
			t.Errorf("the %s row lacks the limit %q or its rewrite %q", row.reader, row.limit, row.resolve)
		}
	}
}
