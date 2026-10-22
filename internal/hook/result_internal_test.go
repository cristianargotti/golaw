package hook

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// TestDecisionResultKeepsTheEngineReason holds a denial over an unmeasured
// check to the engine's English reason byte for byte, marked as a verdict
// golaw could not judge rather than a finding.
func TestDecisionResultKeepsTheEngineReason(t *testing.T) {
	t.Parallel()
	got := decisionResult(model.Event{}, engine.Decision{
		Verdict: engine.VerdictDeny, Reason: "check: not measured: unreadable",
		Unmeasured: []engine.Unmeasured{{Check: "check", Reason: "unreadable"}},
	}, model.DefaultLanguage)
	if !got.CannotJudge || got.Reason != "check: not measured: unreadable" {
		t.Fatal(got)
	}
}
