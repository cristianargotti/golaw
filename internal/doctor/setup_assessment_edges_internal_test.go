package doctor

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestSetupHookFailureStatesPreserveCertainty(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, failure, state string
	}{
		{"nonregular", "hook is not a regular file", "unknown"},
		{"unreadable", "permission denied", "unknown"},
		{"missing fixture", "missing", "missing"},
		{"missing file", "missing: no such file or directory", "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts := setupBaseline()
			facts.Hooks[0].Error = tc.failure
			requireSetupLine(t, facts, collectCase{id: "doctor.setup-hooks", want: tc.state})
		})
	}
}

func TestSetupAssessmentNamesMissingEvidence(t *testing.T) {
	t.Parallel()
	facts := setupBaseline()
	facts.Hooks = nil
	requireSetupLine(t, facts, collectCase{id: "doctor.setup-hooks", want: "missing"})
	facts = setupBaseline()
	facts.EffectiveHooksPath = ""
	requireSetupLine(t, facts, collectCase{id: "doctor.setup-routing", want: "unknown"})
}

func TestSetupAssessmentUntrustedReceiptsAnswerOnlyReceipt(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"absent", "invalid"} {
		facts := setupBaseline()
		facts.ReceiptState = state
		lines := AssessSetup(facts)
		if len(lines) != 1 || lines[0].ID != "doctor.setup-receipt" {
			t.Fatalf("untrusted receipt produced other diagnostics: %+v", lines)
		}
	}
}

func TestSetupAssessmentDisabledRungsStayOptional(t *testing.T) {
	t.Parallel()
	facts := setupBaseline()
	facts.Ladder = model.Ladder{}
	disabled := make(map[string]bool)
	for _, line := range AssessSetup(facts) {
		switch line.State {
		case "disabled":
			disabled[line.Key] = true
		case "ok":
		default:
			t.Fatalf("disabled rung created an adverse diagnostic: %+v", line)
		}
	}
	for _, name := range []string{"pre-commit", "commit-msg", "pre-push"} {
		if !disabled[name] {
			t.Fatalf("missing disabled rung %s: %v", name, disabled)
		}
	}
}
