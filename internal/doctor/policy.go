package doctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/model"
)

// EnrollmentReader keeps policy authority outside diagnostics.
type EnrollmentReader interface {
	ReadEnrollment(
		ctx context.Context,
		identity, policyDigest, profileDigest string,
	) (model.Enrollment, error)
}

// codeVersionUnavailable keeps a version that was never read apart from a
// failed floor: evidence that was not measured cannot fail a requirement.
const codeVersionUnavailable = "agent-version-unavailable"

// The host versions of docs/agent-contracts.md: the floor onFailure: block
// needs, and the version the adapter was validated with.
const (
	versionFloor     = "2.1.295"
	versionValidated = "2.1.299"
)

// versionLines compares the host with the floor and the validated version
// (section 9): a newer host is unknown, so doctor warns before an upgrade
// that nothing proved the adapter against it yet.
func versionLines(snapshot Snapshot) []Line {
	line := Line{
		ID: "doctor.version", State: stateOK, Code: "agent-version-supported",
		Detail: fmt.Sprintf("%s; floor %s, validated %s", snapshot.ClaudeVersion, versionFloor, versionValidated),
	}
	allowed, err := VersionAtLeast(snapshot.ClaudeVersion, versionFloor)
	newer, _ := VersionAtLeast(versionValidated, snapshot.ClaudeVersion) //nolint:errcheck // the floor comparison reports a version that does not parse
	switch {
	case snapshot.ClaudeVersionError != "":
		line.State, line.Code, line.Detail = stateUnknown, codeVersionUnavailable, snapshot.ClaudeVersionError
	case err != nil:
		line.State, line.Code = stateUnknown, codeVersionUnavailable
		line.Detail = fmt.Sprintf("%q: %v", snapshot.ClaudeVersion, err)
	case !allowed:
		line.State, line.Code = stateRed, "agent-version-below-floor"
	case !newer:
		line.State, line.Code = stateUnknown, "agent-version-unvalidated"
		line.Detail += "; newer than the validated host, see docs/agent-contracts.md"
	}
	return []Line{line}
}

// pinLines judges the pin with the release rule of A10: a binary without an
// exact release version skips the pin, as scan and the hook do, so only two
// different releases are red.
func pinLines(snapshot Snapshot) []Line {
	installed := strings.TrimPrefix(snapshot.Receipt.Version, "v")
	pinned := strings.TrimPrefix(snapshot.PinnedVersion, "v")
	line := Line{
		ID: "doctor.pin", State: stateOK, Code: "kit-version-matches",
		Detail: fmt.Sprintf("installed=%s pinned=%s", installed, pinned),
	}
	switch {
	case installed != "" && !buildinfo.IsRelease(installed):
		line.State, line.Code = stateUnknown, "kit-pin-not-enforced"
		line.Detail = "development build, pin not enforced: " + line.Detail
	case installed == "" || installed != pinned:
		line.State, line.Code = stateRed, "kit-version-differs"
	}
	return []Line{line}
}

func buildLines(snapshot Snapshot) []Line {
	build := snapshot.Build
	line := Line{
		ID: "doctor.build", State: "source-build", Code: "source-build-unverifiable",
		Detail: fmt.Sprintf("version=%s sum=%s revision=%s; executable not verifiable",
			build.Version, build.Sum, build.Revision),
	}
	if build == (BuildMetadata{}) {
		line.State, line.Code = stateUnknown, "build-metadata-unavailable"
	}
	return []Line{line}
}

func enrollmentLines(snapshot Snapshot) []Line {
	enrollment := snapshot.Enrollment
	line := Line{
		ID: "doctor.enrollment", State: stateOK,
		Code: enrollment.State.String(), Detail: enrollment.State.String(),
	}
	if snapshot.EnrollmentError != "" ||
		(enrollment.State != model.EnrollmentEnrolled && enrollment.State != model.EnrollmentNone) {
		line.State, line.Code = stateRed, "enrollment-invalid"
		line.Detail += " " + snapshot.EnrollmentError
	}
	return []Line{line}
}
