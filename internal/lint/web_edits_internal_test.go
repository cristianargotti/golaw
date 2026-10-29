package lint

import "testing"

// The first half of the typical edits an agent makes in a script, each a
// sonarjs rule of the README rows (survey 2.3). S125 is not here: it joins
// only when a profile snapshot activates it (LWB-7).
func TestWebTypicalEditRows(t *testing.T) {
	t.Parallel()
	assertWebRows(t, []webRowCase{
		{"javascript:S1481", eslintTool, "sonarjs/no-unused-vars", "exact"},
		{"javascript:S1854", eslintTool, "sonarjs/no-dead-store", "exact"},
		{"javascript:S2486", eslintTool, "sonarjs/no-ignored-exceptions", "exact"},
		{"javascript:S1135", eslintTool, "sonarjs/todo-tag", "exact"},
		{"javascript:S4144", eslintTool, "sonarjs/no-identical-functions", "exact"},
		{"javascript:S1871", eslintTool, "sonarjs/no-duplicated-branches", "exact"},
		{"javascript:S2589", eslintTool, "sonarjs/no-gratuitous-expressions", "exact"},
		{"javascript:S4624", eslintTool, "sonarjs/no-nested-template-literals", "exact"},
	})
}
