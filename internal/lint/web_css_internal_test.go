package lint

import "testing"

// The typical CSS rows: close until a profile snapshot confirms the key to
// rule pairing (survey 6 item 15, M6.12 risk 7).
func TestWebCSSRows(t *testing.T) {
	t.Parallel()
	assertWebRows(t, []webRowCase{
		{"css:S4647", stylelintTool, "color-no-invalid-hex", "close"},
		{"css:S4648", stylelintTool, "font-family-no-duplicate-names", "close"},
		{"css:S4649", stylelintTool, "font-family-no-missing-generic-family-keyword", "close"},
		{"css:S4654", stylelintTool, "property-no-unknown", "close"},
		{"css:S4656", stylelintTool, "declaration-block-no-duplicate-properties", "close"},
		{"css:S4657", stylelintTool, "declaration-block-no-shorthand-property-overrides", "close"},
		{"css:S4658", stylelintTool, "block-no-empty", "close"},
		{"css:S4659", stylelintTool, "selector-pseudo-class-no-unknown", "close"},
		{"css:S4660", stylelintTool, "selector-pseudo-element-no-unknown", "close"},
		{"css:S4662", stylelintTool, "at-rule-no-unknown", "close"},
		{"css:S4663", stylelintTool, "comment-no-empty", "close"},
		{"css:S4664", stylelintTool, "no-descending-specificity", "close"},
		{"css:S4667", stylelintTool, "no-empty-source", "close"},
		{"css:S4668", stylelintTool, "unit-no-unknown", "close"},
		{"css:S5362", stylelintTool, "function-calc-no-unspaced-operator", "close"},
	})
}
