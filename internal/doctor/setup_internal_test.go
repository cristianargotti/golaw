package doctor

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/setup"
)

func setupBaseline() SetupFacts {
	return SetupFacts{
		ReceiptPath:  "state/setup/receipt.json",
		ReceiptState: "present",
		Receipt: setup.Receipt{
			Schema: setup.ReceiptSchema, Root: "root", Identity: "identity",
			HooksPath: "root/hooks",
			Hooks:     []install.Artifact{{Path: "root/hooks/pre-commit", SHA256: "expected"}},
		},
		EffectiveHooksPath: "root/hooks",
		Hooks:              []SetupHook{{Path: "root/hooks/pre-commit", SHA256: "expected"}},
		Tools:              []ToolStatus{{Name: "lefthook", Path: "bin/lefthook", Version: "2.1.12"}},
	}
}

func TestSetupDiagnosticsUseCollectedEvidence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, patch, id, state string
	}{
		{"matching", `{}`, "doctor.setup-hooks", "ok"},
		{"not armed", `{"receipt_state":"absent"}`, "doctor.setup-receipt", "not-armed"},
		{"invalid receipt", `{"receipt_state":"invalid","receipt_error":"invalid JSON"}`, "doctor.setup-receipt", "invalid"},
		{"changed hook", `{"hooks":[{"path":"root/hooks/pre-commit","sha256":"changed"}]}`, "doctor.setup-hooks", "drift"},
		{"missing hook", `{"hooks":[{"path":"root/hooks/pre-commit","error":"missing"}]}`, "doctor.setup-hooks", "missing"},
		{"changed routing", `{"local_hooks_path":"custom","effective_hooks_path":"root/custom"}`, "doctor.setup-routing", "drift"},
		{"same routing", `{"effective_hooks_path":"root/hooks"}`, "doctor.setup-routing", "ok"},
		{"missing tool", `{"tools":[{"name":"lefthook","error":"not on PATH"}]}`, "doctor.setup-tools", "missing"},
		{"other version", `{"tools":[{"name":"lefthook","path":"bin/lefthook","version":"2.1.11"}]}`, "doctor.setup-tools", "version"},
		{"pinned version", `{"tools":[{"name":"lefthook","path":"bin/lefthook","version":"2.1.12"}]}`, "doctor.setup-tools", "ok"},
		{"probe failed", `{"tools":[{"name":"lefthook","path":"bin/lefthook","error":"probe failed"}]}`, "doctor.setup-tools", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lines := patchedAssessment(t, tc.patch)
			for _, line := range lines {
				if line.ID == tc.id && line.State == tc.state {
					return
				}
			}
			t.Fatalf("missing %s %s in %+v", tc.id, tc.state, lines)
		})
	}
}

// patchedAssessment assesses the baseline facts under a JSON patch, and
// requires the same assessment of the facts after a JSON round trip.
func patchedAssessment(t *testing.T, patch string) []Line {
	t.Helper()
	facts := setupBaseline()
	if err := json.Unmarshal([]byte(patch), &facts); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	var replay SetupFacts
	if unmarshalErr := json.Unmarshal(encoded, &replay); unmarshalErr != nil {
		t.Fatal(unmarshalErr)
	}
	lines := AssessSetup(facts)
	if !reflect.DeepEqual(lines, AssessSetup(replay)) {
		t.Fatal("serialized evidence changed the assessment")
	}
	return lines
}
