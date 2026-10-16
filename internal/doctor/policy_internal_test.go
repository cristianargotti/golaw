package doctor

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestEnrollmentStatesAndBuildAssurance(t *testing.T) {
	t.Parallel()
	for _, state := range []model.EnrollmentState{
		model.EnrollmentNone, model.EnrollmentEnrolled,
		model.EnrollmentMismatch, model.EnrollmentMissing,
	} {
		input := baseline(t)
		input.Enrollment.State = state
		want := stateOK
		if state == model.EnrollmentMismatch || state == model.EnrollmentMissing {
			want = stateRed
		}
		if enrollmentLine(t, input) != want {
			t.Fatalf("enrollment state not reported: %s", state.String())
		}
	}
	input := baseline(t)
	input.Receipt.Version = "0.1.1"
	for _, line := range Assess(input).Lines {
		if line.ID == "doctor.pin" && line.State != "red" {
			t.Fatal("newer installed version matched an older pin")
		}
	}
}

// enrollmentLine answers the state of the enrollment line and checks the
// build line stays a source build.
func enrollmentLine(t *testing.T, input Snapshot) string {
	t.Helper()
	state := ""
	for _, line := range Assess(input).Lines {
		if line.ID == "doctor.enrollment" {
			state = line.State
		}
		if line.ID == "doctor.build" && line.State != "source-build" {
			t.Fatalf("source metadata has wrong state: %#v", line)
		}
	}
	return state
}

func TestPythonArgumentDoesNotBecomeAnInvocation(t *testing.T) {
	t.Parallel()
	input := baseline(t)
	input.Settings = `{"hooks":{"PreToolUse":[{"matcher":"Read","hooks":[{"type":"command","command":"echo python3 gate.py"}]}]}}`
	lines := pythonLines(input)
	if len(lines) != 1 || lines[0].State != "ok" {
		t.Fatalf("harmless argument treated as a gate: %#v", lines)
	}
}
