package cicheck_test

import (
	"regexp"
	"strings"
	"testing"
)

// TestChangelogDatesThePolicyRules holds the dated line that names every
// rule reference of the policy family, written before any check of the
// family registers, so TestChangelogDatesEveryRule holds the moment a half
// of the family registers its first check.
func TestChangelogDatesThePolicyRules(t *testing.T) {
	t.Parallel()
	dated := regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: the policy family, .*, because .*\.$`)
	line := dated.FindString(string(kitBytes(t, "CHANGELOG.md")))
	if line == "" {
		t.Fatal("no dated line names the policy family with its reason")
	}
	for _, rule := range []string{
		"rules/4.5@1", "rules/4.6@1", "rules/4.7@1", "rules/4.8@1",
		"rules/4.9@1", "rules/4.10@1", "rules/4.12@1", "rules/4.13@1",
	} {
		if !strings.Contains(line, rule+" ") && !strings.Contains(line, rule+",") {
			t.Errorf("the dated line of the policy family does not name %s", rule)
		}
	}
}
