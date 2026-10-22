package cicheck_test

import "testing"

// TestStartingStatesGolawDoesNotSupportAreStated pins each starting state
// the template survey found the kit cannot adopt as it stands, with what
// the person or the platform does about it.
func TestStartingStatesGolawDoesNotSupportAreStated(t *testing.T) {
	t.Parallel()
	requireText(t, "docs/setup.md",
		"## Starting states golaw does not support",
		"When CODEOWNERS gives `.github/` to a team other than the catch-all owner, init writes no Actions file and no Dependabot file there",
		"the generated CI and the Dependabot cooldown stay out of the repository, and the remote gate is whatever the platform runs",
		"A container test that needs a private registry or a cloud login is outside the kit: the `test` target of `golaw.mk` runs `go test` on the machine and needs neither.",
		"A Swagger 2.0 tree a generator wrote is inherited state the kit does not regenerate: the API path of the kit is OpenAPI 3 with oapi-codegen",
		"`policy.openapi-assertions` judges no Swagger 2.0 document and measures clean until the OpenAPI 3 contract `redocly.yaml` names exists",
		"A go line past Go's support window adopts, and its standard library advisories close only through a go line bump",
		"the `vuln` target and the push rung need dated entries in `vulnerability_waivers` until the line moves; the kit never moves it.",
		"A repository born with lint debt is red on its first `make ci` until the debt is fixed: M3 ships no lint baseline",
		"A generator that rewrites a file without the Go generated header flips with `make fmt`",
	)
}
