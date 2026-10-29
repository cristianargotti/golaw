package lint

import "testing"

// The rows of the hub issue classes measured in survey 1.2 and 2.2.
func TestWebHubRows(t *testing.T) {
	t.Parallel()
	assertWebRows(t, []webRowCase{
		{"javascript:S2681", eslintTool, "sonarjs/no-unenclosed-multiline-block", "exact"},
		{"javascript:S1128", eslintTool, "sonarjs/unused-import", "exact"},
		{"javascript:S6479", eslintTool, "react/no-array-index-key", "exact"},
		{"javascript:S3358", eslintTool, "sonarjs/no-nested-conditional", "exact"},
		{"javascript:S3776", eslintTool, "sonarjs/cognitive-complexity", "exact"},
		{"javascript:S7764", eslintTool, "unicorn/prefer-global-this", "exact"},
		{"javascript:S6819", eslintTool, "jsx-a11y/prefer-tag-over-role", "exact"},
		{"javascript:S930", eslintTool, "sonarjs/no-extra-arguments", "exact"},
		{"javascript:S1523", eslintTool, "sonarjs/code-eval", "exact"},
		{"css:S4666", stylelintTool, "no-duplicate-selectors", "exact"},
	})
}
