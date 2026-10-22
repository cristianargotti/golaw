package cicheck_test

import "testing"

// TestM3AcceptancePageNamesItsRowsAndLimits holds the M3 evidence page to
// the rows the m3 lane requires and to the limits it claims nothing past
// (A23): every category, the six named cases, the two measured dependency
// rows, the command that reproduces them, the systems each row is required
// on and the pipeline as the only gate.
func TestM3AcceptancePageNamesItsRowsAndLimits(t *testing.T) {
	t.Parallel()
	const page = "docs/acceptance/m3.md"
	requireText(t, page,
		"# M3 acceptance",
		"go run ./tools/acceptance -m3",
		"m3-policy",
		"m3-cases",
		"m3-template",
		"m3-capability",
		"m3-adoption",
		"dependency/internal-replace",
		"dependency/external-replace",
		"disabled-linter",
		"hollowed-by-exclusion",
		"modular-makefile",
		"missing-include",
		"second-require-block",
		"generated-file-allowlist",
		"The m3 matrix uses ubuntu-latest, macos-latest and windows-latest.",
		"The replay of the generated CI is required on ubuntu-latest and an optional SKIP elsewhere; a SKIP never counts as PASS.",
		"Each policy row runs make -k ci, so the scan target reports its violation even where an earlier capability target fails for a tool the machine lacks.",
		"Local checks remain optional. Only remote CI or the adopting platform pipeline is a hard gate.",
	)
	if homePath.Match(kitBytes(t, page)) {
		t.Fatal("the M3 acceptance page names a machine path")
	}
}
