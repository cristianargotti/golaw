package lint

import "testing"

// The second half of the typical edits; S3800, S1874 and S6759 read the
// types of the kit program.
func TestWebTypicalEditRowsSecondHalf(t *testing.T) {
	t.Parallel()
	assertWebRows(t, []webRowCase{
		{"javascript:S2004", eslintTool, "sonarjs/no-nested-functions", "exact"},
		{"javascript:S3800", eslintTool, "sonarjs/function-return-type", "exact"},
		{"javascript:S5843", eslintTool, "sonarjs/regex-complexity", "exact"},
		{"javascript:S6353", eslintTool, "sonarjs/concise-regex", "exact"},
		{"javascript:S2245", eslintTool, "sonarjs/pseudo-random", "exact"},
		{"javascript:S5725", eslintTool, "sonarjs/disabled-resource-integrity", "exact"},
		{"javascript:S1874", eslintTool, "sonarjs/deprecation", "exact"},
		{"javascript:S6759", eslintTool, "sonarjs/prefer-read-only-props", "exact"},
	})
}
