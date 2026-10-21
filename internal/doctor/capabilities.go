package doctor

import "strings"

// The line that names the capabilities of the policy in force, which
// choose the checks, the rung steps and the templates that apply.
const (
	idCapabilities   = "doctor.capabilities"
	codeCapabilities = "capabilities-in-force"
)

// AssessCapabilities is the line that names the capabilities in force,
// comma and space separated, or none: a fact the person reads before
// asking why a check, a rung step or a template does not apply, never a
// diagnosis, so its state is always ok.
func AssessCapabilities(capabilities []string) Line {
	detail := strings.Join(capabilities, ", ")
	if detail == "" {
		detail = "none"
	}
	return Line{ID: idCapabilities, Key: "capabilities", State: stateOK, Code: codeCapabilities, Detail: detail}
}
