package cicheck_test

import (
	"regexp"
	"strings"
	"testing"
)

func m4ChangeNotesOK(text string, expressions []*regexp.Regexp, forbidden []string) bool {
	_, unreleased, found := strings.Cut(text, "## Unreleased\n")
	if !found {
		return false
	}
	unreleased, _, _ = strings.Cut(unreleased, "\n## ")
	_, added, found := strings.Cut(unreleased, "### Added\n")
	if !found {
		return false
	}
	added, _, _ = strings.Cut(added, "\n### ")
	for _, expression := range expressions {
		if !expression.MatchString(added) {
			return false
		}
	}
	return m4TextOK(text, nil, forbidden)
}

func TestM4DatedChangeNotes(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "CHANGELOG.md"))
	notes := []string{
		"M4 adds a patch adapter pinned to 0.163.0 and bounded shell routes, because supported edits need the shared judgment before execution.",
		"M4 adds a compatibility preview with key capture, read and shell guards and advisory post feedback, because unknown write payloads must remain available without an enforcement claim.",
		"M4 adds per-agent receipts and shared skill ownership for user and project installation, because removing one integration must preserve another integration's skill.",
		"M4 separates installed, trusted and active state in doctor, because a present hook can be inert.",
		"M4 documents assurance per agent, event and operating system and the residual security boundaries in docs/agent-contracts.md, because source contracts, offline invocations and live dispatch prove different things.",
	}
	expressions := make([]*regexp.Regexp, 0, len(notes))
	for _, note := range notes {
		expressions = append(expressions, regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: `+regexp.QuoteMeta(note)+`$`))
	}
	dated := regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: (M4 )`)
	forbidden := []string{
		"M4 proves live enforcement on every operating system.",
		"The compatibility preview enforces unknown writes.",
	}
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-date", dated.ReplaceAllString(text, "- $1"), false},
		{"red-missing-reason", strings.ReplaceAll(text, "because a present hook can be inert.", ""), false},
		{"red-live-overclaim", text + "\n" + forbidden[0], false},
		{"red-preview-overclaim", text + "\n" + forbidden[1], false},
		{"near-historical-entry", text + "\nEarlier evidence remains historical.\n", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m4ChangeNotesOK(row.text, expressions, forbidden); got != row.want {
				t.Fatalf("change notes = %v, want %v", got, row.want)
			}
		})
	}
}
