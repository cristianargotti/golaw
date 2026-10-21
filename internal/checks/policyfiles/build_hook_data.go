package policyfiles

// hookTestRule is one hook script convention the check recognizes: a script
// directly in directory with scriptSuffix wants the sibling that swaps the
// suffix for testSuffix. It is data so a profile convention never needs a
// second evaluator.
type hookTestRule struct {
	directory, scriptSuffix, testSuffix string
}

// hookTestRules are the conventions the tests inventory states; the
// specification defines no discovery for other script languages or test
// frameworks, so none is inferred.
func hookTestRules() []hookTestRule {
	return []hookTestRule{
		{directory: "scripts/hooks", scriptSuffix: ".sh", testSuffix: ".test.sh"},
	}
}
