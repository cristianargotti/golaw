package lint_test

import (
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestToFindingNamesTheToolAndTheRule(t *testing.T) {
	t.Parallel()
	issue := lint.Issue{Tool: "ruff", Rule: "S307", Path: "test/deploy/test_install.py", Line: 25, Column: 12, Message: "Use of possibly insecure function; consider using `ast.literal_eval`", Severity: "error"}
	got := lint.ToFinding(issue)
	want := model.Finding{
		Check: "lint.ruff", Rule: "ruff/S307", Path: issue.Path, Line: 25, Column: 12,
		Code: "s307", Message: issue.Message, Severity: model.SeverityFail,
	}
	want.Fingerprint = model.Fingerprint(want)
	if got.Check != want.Check || got.Rule != want.Rule || got.Code != want.Code || got.Severity != want.Severity || got.Fingerprint != want.Fingerprint || got.Line != 25 {
		t.Errorf("ToFinding = %+v, want %+v", got, want)
	}
	moved := issue
	moved.Line = 40
	if lint.ToFinding(moved).Fingerprint != got.Fingerprint {
		t.Error("a moved line changed the fingerprint, which never holds a line (spec 5.2)")
	}
	suppressed := issue
	suppressed.Suppressed = true
	if note := lint.ToFinding(suppressed); note.Severity != model.SeverityNote || note.Message != issue.Message+lint.SuppressedNote {
		t.Errorf("a suppressed issue = %+v, want a note that says the pipeline may still report it", note)
	}
}

func TestTheLintTimeoutBoundsATargetRunNotAHook(t *testing.T) {
	t.Parallel()
	if lint.ToolTimeout != 10*time.Minute {
		t.Errorf("ToolTimeout = %v", lint.ToolTimeout)
	}
}
