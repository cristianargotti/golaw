package lint

import "testing"

// The external rows: the hub's single findings of survey 1.2 (S1186, S1788,
// S6551, S7758, S7784, S7785) and the web set's (S7761, S7765, S7781, S7787).
func TestWebExternalRows(t *testing.T) {
	t.Parallel()
	assertWebRows(t, []webRowCase{
		{"javascript:S1186", eslintTool, "@typescript-eslint/no-empty-function", "exact"},
		{"javascript:S1788", eslintTool, "@typescript-eslint/default-param-last", "exact"},
		{"javascript:S6551", eslintTool, "@typescript-eslint/no-base-to-string", "exact"},
		{"javascript:S7758", eslintTool, "unicorn/prefer-code-point", "exact"},
		{"javascript:S7784", eslintTool, "unicorn/prefer-structured-clone", "exact"},
		{"javascript:S7785", eslintTool, "unicorn/prefer-top-level-await", "exact"},
		{"javascript:S7761", eslintTool, "unicorn/dom-node-dataset", "exact"},
		{"javascript:S7765", eslintTool, "unicorn/prefer-includes", "exact"},
		{"javascript:S7781", eslintTool, "unicorn/prefer-string-replace-all", "exact"},
		{"javascript:S7787", eslintTool, "unicorn/require-module-specifiers", "exact"},
	})
}
