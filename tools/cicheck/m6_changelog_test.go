package cicheck_test

import (
	"regexp"
	"strings"
	"testing"
)

// m6Notes are the bodies of the dated M6 lines; the line of the surface
// family is held apart by its own expression.
func m6Notes() []string {
	return []string{
		"M6 detects the languages of the other files of a Go repository from its snapshot and reads the analyzer scope from a profile's analyzer.yml, sonar-project.properties or golaw.yml, because the remote analyzer judges every language of the tree while the scope never relaxes a core law.",
		"M6 adds golaw lint with kit-owned parity and strict configurations whose rules map to the Sonar way rules, run with the repository's configuration lookup disabled, because a gate the pipeline fails must be reproducible before the push.",
		"M6 adds golaw setup --surface, the only command that downloads the pinned surface tools, verified by checksum into a tool directory outside the checkout, because no hook may reach the network or run an unverified tool.",
		"M6 runs ruff, hadolint and shellcheck on the file an agent just wrote and feeds their findings back without denying, because a fast linter helps the agent while only a certain check may block it.",
		"M6 adds golaw coverage --new-code with Sonar's line model and the exclusions of the analyzer scope, and refuses a missing or stale report, because the gate failed on a coverage of 0 computed without the report.",
		"M6 changes the policy.image-digest message and the example Dockerfile to a digest without a tag, because a tag beside a digest is the docker:S8431 finding the kit's own example produced.",
		"M6 records its acceptance rows in docs/acceptance/m6.md and the rule map with its gaps in docs/surface.md, because a parity claim names the rule it maps and the keys it leaves to the pipeline.",
	}
}

func TestM6DatedChangeNotes(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "CHANGELOG.md"))
	notes := m6Notes()
	expressions := make([]*regexp.Regexp, 0, len(notes)+1)
	expressions = append(expressions,
		regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: the surface family, rule rules/4\.16@1, .*, because .*\.$`))
	for _, note := range notes {
		expressions = append(expressions, regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: `+regexp.QuoteMeta(note)+`$`))
	}
	forbidden := []string{"golaw calls the Sonar", "M6 proves the Sonar way gate"}
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-date", regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: (M6 )`).ReplaceAllString(text, "- $1"), false},
		{"red-missing-reason", strings.Replace(text, "because no hook may reach the network or run an unverified tool.", "", 1), false},
		{"red-gate-overclaim", text + "\n" + forbidden[1] + ".\n", false},
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
