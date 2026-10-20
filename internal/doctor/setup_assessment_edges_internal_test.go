package doctor

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestSetupReceiptLineNamesTheRecordedGolaw(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, golaw, want string
	}{
		{"unix", "/srv/tools/bin/golaw", "setup receipt is present; setup recorded golaw at /srv/tools/bin/golaw, and the armed hooks run golaw from PATH"},
		{"windows", `C:\tools\bin\golaw.exe`, `setup receipt is present; setup recorded golaw at C:\tools\bin\golaw.exe, and the armed hooks run golaw from PATH`},
		{"unrecorded", "", "setup receipt is present"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts := setupBaseline()
			facts.Receipt.Golaw = tc.golaw
			var out bytes.Buffer
			if err := Print(&out, Report{Schema: 1, Lines: AssessSetup(facts)}, true); err != nil {
				t.Fatal(err)
			}
			var replay Report
			if err := json.Unmarshal(out.Bytes(), &replay); err != nil {
				t.Fatal(err)
			}
			line := replay.Lines[0]
			if line.ID != "doctor.setup-receipt" || line.Key != facts.ReceiptPath || line.State != "ok" ||
				line.Code != "setup-receipt-present" || line.Detail != tc.want {
				t.Fatalf("receipt line = %+v, want detail %q", line, tc.want)
			}
		})
	}
}

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
