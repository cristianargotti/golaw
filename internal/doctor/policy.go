package doctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// EnrollmentReader keeps policy authority outside diagnostics.
type EnrollmentReader interface {
	ReadEnrollment(
		ctx context.Context,
		identity, policyDigest, profileDigest string,
	) (model.Enrollment, error)
}

func versionLines(snapshot Snapshot) []Line {
	line := Line{
		ID: "doctor.version", State: stateOK, Code: "agent-version-supported",
		Detail: snapshot.ClaudeVersion,
	}
	allowed, err := VersionAtLeast(snapshot.ClaudeVersion, "2.1.295")
	if err != nil || !allowed {
		line.State, line.Code = stateRed, "agent-version-below-floor"
	}
	return []Line{line}
}

func pinLines(snapshot Snapshot) []Line {
	installed := strings.TrimPrefix(snapshot.Receipt.Version, "v")
	pinned := strings.TrimPrefix(snapshot.PinnedVersion, "v")
	line := Line{
		ID: "doctor.pin", State: stateOK, Code: "kit-version-matches",
		Detail: fmt.Sprintf("installed=%s pinned=%s", installed, pinned),
	}
	if installed == "" || installed != pinned {
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
