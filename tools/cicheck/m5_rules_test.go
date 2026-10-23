package cicheck_test

import (
	"strings"
	"testing"
)

const (
	m5RulesPath   = "docs/rules.md"
	m5ProfileRule = "rules/4.14@1"
	m5ProfileHead = "\n## " + m5ProfileRule + "\n"
	m5ReadersHead = "\n## Reader subsets and stated limits\n"
)

// m5ProfileSentences are the claims of the 4.14 section a reader relies on:
// the evaluators read data, answer NotApplicable without their section, and
// the inventory rows of 4.14 stay out of the prose mapping.
func m5ProfileSentences() []string {
	return []string{
		"never with code the bundle carries",
		"Each answers NotApplicable without a profile and when the profile carries no section for it",
		"A profile only adds checks; its schema has no key that names a core floor.",
		"Inventory section 4.14 is profile data, so the prose mapping holds no 4.14 row",
		"The forbidden paths of a profile are paths.profile-forbidden under rules/4.2@1",
	}
}

// m5ProfileSection answers the body of the 4.14 section when it stands
// between the 4.13 section and the reader subsets, the order of the page.
func m5ProfileSection(text string) (string, bool) {
	before := strings.Index(text, "\n## rules/4.13@1\n")
	at := strings.Index(text, m5ProfileHead)
	after := strings.Index(text, m5ReadersHead)
	if before < 0 || at < before || after < at {
		return "", false
	}
	body, _, _ := strings.Cut(text[at+len(m5ProfileHead):], "\n## ")
	return body, true
}

// m5ProfileRulesOK holds the section to its place, its sentences and its
// rows: every check row of the section carries rule 4.14, and no prose
// mapping row of the page names 4.14.
func m5ProfileRulesOK(text string) bool {
	body, found := m5ProfileSection(text)
	if !found || strings.Contains(text, "\n| 4.14") || !m4TextOK(body, m5ProfileSentences(), nil) {
		return false
	}
	for line := range strings.SplitSeq(body, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 6 {
			continue
		}
		rule := strings.TrimSpace(cells[2])
		if strings.HasPrefix(rule, "rules/") && rule != m5ProfileRule {
			return false
		}
	}
	return true
}

func TestM5ProfileRuleSection(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m5RulesPath))
	row := "| policy.image-registry | rules/4.14@1 | image | An image outside the pattern, because the registry is the platform's. |\n"
	foreign := "| policy.makefile | rules/4.13@1 | go | A Makefile without its targets, because the pipeline runs them. |\n"
	moved := strings.Replace(text, m5ProfileHead, "\n## Profile evaluators\n", 1) +
		m5ProfileHead + "\n" + strings.Join(m5ProfileSentences(), " ") + "\n"
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-heading", strings.Replace(text, m5ProfileHead, "\n## rules/4.14\n", 1), false},
		{"red-after-the-readers", moved, false},
		{"red-prose-row", text + "\n| 4.14: platform values | docs/profiles.md |\n", false},
		{"red-foreign-row", strings.Replace(text, m5ReadersHead, "\n"+foreign+m5ReadersHead, 1), false},
		{"red-missing-floor-sentence", strings.Replace(text, m5ProfileSentences()[2], "", 1), false},
		{"near-profile-row", strings.Replace(text, m5ReadersHead, "\n"+row+m5ReadersHead, 1), true},
		{"near-paths-row-elsewhere", text + "\n| paths.profile-forbidden | rules/4.2@1 | none | A path, because. |\n", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m5ProfileRulesOK(row.text); got != row.want {
				t.Fatalf("profile rule section = %v, want %v", got, row.want)
			}
		})
	}
	if got := lineCount(kitBytes(t, m5RulesPath)); got > 300 {
		t.Fatalf("%s has %d lines, maximum 300", m5RulesPath, got)
	}
}
