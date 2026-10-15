package install

import (
	"fmt"
	"slices"
	"strings"
)

// ReadRules adapts the shared name data without introducing another matcher.
func ReadRules(globs []string) ([]string, error) {
	rules := make([]string, 0, len(globs))
	for _, glob := range globs {
		if glob == "" || strings.ContainsAny(glob, "()\r\n") ||
			strings.HasSuffix(strings.ToLower(glob), ".go") {
			return nil, fmt.Errorf("secret glob %q: %w", glob, ErrSettings)
		}
		rule := "Read(" + glob + ")"
		if !slices.Contains(rules, rule) {
			rules = append(rules, rule)
		}
	}
	return rules, nil
}
