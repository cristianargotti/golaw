package cicheck_test

import (
	"regexp"
	"strings"
	"testing"
)

// m5Notes are the bodies of the dated M5 lines; the evaluator line of the
// loader part is held apart by its own expression.
func m5Notes() []string {
	return []string{
		"M5 installs a profile bundle from its git source at a tag with golaw profile install, verifying its stamp, digest and schema before it writes, and keeps every installed version in a store with refresh, rollback and remove, because a profile is data a person installs and two repositories may pin two versions of it.",
		"M5 loads the pinned bundle in every governed operation and refuses a missing, stale or mismatched bundle with one line naming the exact install command, because a fresh machine must never fall back to core silently or deny without a remedy.",
		"M5 adds init --profile, which pins the installed stamp and leaves the destinations a profile names to the platform, and init --seed-debt, which seeds the reviewed debt set once, because a pin and the debt change only through a reviewed commit.",
		"M5 adds doctor --release, which verifies a downloaded binary against the signed checksums.txt, its Sigstore bundle and the provenance attestation under the pinned identity and exits 1 on a red result, because a person decides whether to trust a binary before running it.",
		"M5 moves the decision of the release gate into tools/releasegate, replayable as make release-gate-dry-run, because a release from a commit without green CI must be refused by a dry run anyone can repeat.",
		"M5 records its acceptance rows in docs/acceptance/m5.md and its distribution residuals in docs/security.md, because a profile bundle on disk is trusted by its digest alone and the inclusion proof of a release is not verified.",
	}
}

func TestM5DatedChangeNotes(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "CHANGELOG.md"))
	notes := m5Notes()
	expressions := make([]*regexp.Regexp, 0, len(notes)+1)
	expressions = append(expressions,
		regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: the profile evaluators, rule rules/4\.14@1, .*, because .*\.$`))
	for _, note := range notes {
		expressions = append(expressions, regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: `+regexp.QuoteMeta(note)+`$`))
	}
	dated := regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: (M5 )`)
	forbidden := []string{
		"M5 proves a remote hard gate",
		"doctor --release verifies a source build",
	}
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-date", dated.ReplaceAllString(text, "- $1"), false},
		{"red-missing-reason", strings.Replace(text, "because a person decides whether to trust a binary before running it.", "", 1), false},
		{"red-missing-evaluator-line", strings.Replace(text, "the profile evaluators, rule rules/4.14@1", "the evaluators", 1), false},
		{"red-remote-overclaim", text + "\n" + forbidden[0] + ".\n", false},
		{"red-source-build-overclaim", text + "\n" + forbidden[1] + ".\n", false},
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
