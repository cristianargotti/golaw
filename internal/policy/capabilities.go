package policy

import "slices"

// goCapability is the capability of a repository that holds a Go program.
const goCapability = "go"

// goShapes are the capabilities that describe the shape of a Go program,
// in the order a refusal names the first one found: each needs go, while
// go and image stand alone.
func goShapes() []string { return []string{"api", "cli", "db", "hexagonal"} }

// CheckCapabilities refuses a capability set that enables the shape of a Go
// program without go, naming the first such capability and the fix, so a
// hand-written policy whose Go checks would never run is refused at load
// instead of judging the repository with them silently off.
func CheckCapabilities(capabilities []string) error {
	if slices.Contains(capabilities, goCapability) {
		return nil
	}
	for _, name := range goShapes() {
		if slices.Contains(capabilities, name) {
			return invalid("capability %s needs go: add go to the capabilities", name)
		}
	}
	return nil
}
