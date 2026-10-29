package cicheck_test

import (
	"regexp"
	"strings"
	"testing"
)

const (
	m6RulesPath   = "docs/rules.md"
	m6SurfaceRule = "rules/4.16@1"
)

// m6SurfaceSection is the text of the surface section up to the next
// heading of its level.
func m6SurfaceSection(text string) (string, bool) {
	_, section, found := strings.Cut(text, "\n## "+m6SurfaceRule+"\n")
	if !found {
		return "", false
	}
	section, _, _ = strings.Cut(section, "\n## ")
	return section, true
}

// m6SurfaceOK holds the section: its intro, its table header, and every
// row it carries under the surface rule with a stated why.
func m6SurfaceOK(text string) bool {
	section, found := m6SurfaceSection(text)
	if !found || !m4TextOK(section, []string{
		"The surface family judges the non-Go files of a Go repository",
		"A certain check denies only a violation the candidate introduces",
		"a feedback check reports and never denies",
		"docs/surface.md holds the rule map to the remote analyzer and the gaps.",
		"| Check | Rule | Needs | Why and boundary |",
	}, nil) {
		return false
	}
	for line := range strings.SplitSeq(section, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 6 || !strings.HasPrefix(strings.TrimSpace(cells[2]), "rules/") {
			continue
		}
		if strings.TrimSpace(cells[2]) != m6SurfaceRule || !strings.Contains(cells[4], "because") {
			return false
		}
	}
	return true
}

// m6WithSurfaceRow inserts a row right after the separator line of the
// surface table, wherever the section stands and whatever rows it holds.
func m6WithSurfaceRow(text, row string) string {
	head, section, found := strings.Cut(text, "\n## "+m6SurfaceRule+"\n")
	if !found {
		return text
	}
	const separator = "| --- | --- | --- | --- |\n"
	before, after, found := strings.Cut(section, separator)
	if !found {
		return text
	}
	return head + "\n## " + m6SurfaceRule + "\n" + before + separator + row + after
}

func TestM6SurfaceRuleIsDocumentedAndDated(t *testing.T) {
	t.Parallel()
	rules := string(kitBytes(t, m6RulesPath))
	dated := regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: the surface family, rule rules/4\.16@1, .*, because .*\.$`)
	changelog := string(kitBytes(t, "CHANGELOG.md"))
	if !dated.MatchString(changelog) {
		t.Error("CHANGELOG.md dates no line of the surface family under rules/4.16@1")
	}
	near := "| surface.sample | rules/4.16@1 | none | A sample row, because a row carries its why. |\n"
	other := "| surface.sample | rules/4.3@1 | none | A sample row, because a row carries its why. |\n"
	for _, tc := range []struct {
		name, text string
		want       bool
	}{
		{"repository", rules, true},
		{"near a row with its why", m6WithSurfaceRow(rules, near), true},
		{"red no heading", strings.Replace(rules, "## "+m6SurfaceRule+"\n", "## rules/4.17@1\n", 1), false},
		{"red no intro", strings.Replace(rules, "A certain check denies only a violation the candidate introduces", "", 1), false},
		{"red a row under another rule", m6WithSurfaceRow(rules, other), false},
		{"red a row without its why", m6WithSurfaceRow(rules, strings.Replace(near, "because", "since", 1)), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := m6SurfaceOK(tc.text); got != tc.want {
				t.Fatalf("surface section = %v, want %v", got, tc.want)
			}
		})
	}
}
