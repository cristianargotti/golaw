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
	for _, line := range assess(input).Lines {
		if line.ID == "doctor.pin" && line.State != "red" {
			t.Fatal("newer installed version matched an older pin")
		}
	}
}

// enrollmentLine also holds the build line to source-build, because no
// enrollment state may turn a source build into a verified one.
func enrollmentLine(t *testing.T, input Snapshot) string {
	t.Helper()
	state := ""
	for _, line := range assess(input).Lines {
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

func TestPinRequiresAnInstalledVersion(t *testing.T) {
	t.Parallel()
	input := baseline(t)
	input.Receipt.Version, input.PinnedVersion = "", ""
	if line := pinLines(input)[0]; line.State != stateRed || line.Code != "kit-version-differs" {
		t.Fatalf("an empty installed version matched an empty pin: %#v", line)
	}
	input.Receipt.Version, input.PinnedVersion = "v0.1.0", "0.1.0"
	if line := pinLines(input)[0]; line.State != stateOK {
		t.Fatalf("a tag prefix broke the pin: %#v", line)
	}
}

// TestBuildLineNamesTheVerificationOfAReleaseVersion pins the wording of
// the build line: a release version without a module sum, as the release
// lane builds it, names doctor --release, which verifies it, while a go
// install build and a pseudo-version stay not verifiable; the state and
// the code stay those of a source build either way.
func TestBuildLineNamesTheVerificationOfAReleaseVersion(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		build BuildMetadata
		want  string
	}{
		"release lane": {BuildMetadata{Version: "v0.5.0", Revision: "r"}, "version=v0.5.0 sum= revision=r; release version, executable verified only by golaw doctor --release <dir>"},
		"go install":   {BuildMetadata{Version: "v0.5.0", Sum: "h1:x", Revision: "r"}, "version=v0.5.0 sum=h1:x revision=r; executable not verifiable"},
		"pseudo":       {BuildMetadata{Version: "v0.0.0-20261023112200-767ca1bb937b", Revision: "r"}, "version=v0.0.0-20261023112200-767ca1bb937b sum= revision=r; executable not verifiable"},
	}
	for name, tc := range cases {
		line := buildLines(Snapshot{Build: tc.build})[0]
		if line.Detail != tc.want || line.State != "source-build" || line.Code != "source-build-unverifiable" {
			t.Errorf("%s: %#v, want detail %q", name, line, tc.want)
		}
	}
}
