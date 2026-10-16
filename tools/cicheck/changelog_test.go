package cicheck_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
)

// TestChangelogDatesEveryRule holds CHANGELOG.md to the promise of its
// first lines (section 4.6): every rule reference the catalog ships is
// named on a dated entry, so a family that ships without its line fails.
func TestChangelogDatesEveryRule(t *testing.T) {
	t.Parallel()
	dated := regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: .*$`)
	entries := strings.Join(dated.FindAllString(string(kitBytes(t, "CHANGELOG.md")), -1), "\n")
	for _, c := range checks.Catalog() {
		spec := c.Spec()
		if !strings.Contains(entries, spec.Rule) {
			t.Errorf("%s ships rule %s, which no dated changelog entry names", spec.ID, spec.Rule)
		}
	}
}
