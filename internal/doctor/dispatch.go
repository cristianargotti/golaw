package doctor

import (
	"fmt"
	"time"
)

// dispatchLines is the final line of section 5.6: the last dispatch
// evidence of the agent on its OS. It says that the host ran the hook,
// never that the hook was trusted or enforced; trust keeps its own line.
func dispatchLines(snapshot Snapshot) []Line {
	dispatch := snapshot.Dispatch
	line := Line{
		ID: "doctor.dispatch", State: stateUnknown, Code: "dispatch-unobserved",
		Detail: "no hook process has run for this repository under this home",
	}
	switch {
	case dispatch.State == DispatchStale:
		line.Code = "dispatch-stale"
		line.Detail = fmt.Sprintf("the last hook process, %s on %s, ran an earlier hook definition", stamp(dispatch.SeenAt), dispatch.OS)
	case dispatch.State != DispatchObserved:
		// No evidence: the unobserved line stands.
	case dispatch.SeenAt.IsZero():
		line.Code, line.Detail = "dispatch-undated", "a hook process ran with no time recorded"
	default:
		line.State, line.Code = stateOK, "dispatch-observed"
		line.Detail = fmt.Sprintf("last hook process %s on %s; trust and enforcement are live evidence", stamp(dispatch.SeenAt), dispatch.OS)
	}
	return []Line{line}
}

// stamp writes a time the way every line of the report does, in UTC.
func stamp(at time.Time) string {
	if at.IsZero() {
		return "at an unknown time"
	}
	return at.UTC().Format(time.RFC3339)
}
