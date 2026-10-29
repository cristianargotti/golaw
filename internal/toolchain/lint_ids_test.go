package toolchain_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// TestEveryPinnedToolNamesAValidLintCheck ties the table to the report: a
// tool's findings carry the check lint.<tool>, so a tool name the check id
// grammar refuses would break every outcome of that tool.
func TestEveryPinnedToolNamesAValidLintCheck(t *testing.T) {
	t.Parallel()
	tools, err := toolchain.Table()
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		f := lint.ToFinding(lint.Issue{Tool: tool.Name, Rule: "R1", Message: "m"})
		if !model.ValidID(f.Check) || f.Rule != tool.Name+"/R1" {
			t.Errorf("%s: finding check %q, rule %q", tool.Name, f.Check, f.Rule)
		}
	}
}
