package report

import (
	"slices"
	"testing"
)

// TestPhrasesHoldThePolicyHalves pins the split of the phrase table: every
// sentence of the lint, build, dependency and profile halves of the policy family is
// a row of the table the human channel reads, and no code has two
// sentences, so two halves that pick one code are refused here.
func TestPhrasesHoldThePolicyHalves(t *testing.T) {
	t.Parallel()
	all := phrases()
	for _, half := range [][]sentence{policyLintPhrases(), policyBuildPhrases(), policyDepsPhrases(), policyProfilePhrases()} {
		for _, s := range half {
			if !slices.Contains(all, s) {
				t.Errorf("the sentence of %s is not in the table", s.code)
			}
		}
	}
	seen := make(map[string]bool, len(all))
	for _, s := range all {
		if seen[s.code] {
			t.Errorf("code %s has two sentences", s.code)
		}
		seen[s.code] = true
	}
}
