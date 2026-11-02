package cicheck_test

import (
	"regexp"
	"strings"
	"testing"
)

func m7Notes() []string {
	return []string{
		"M7 adds golaw new, which writes a port, an adapter over SQL or HTTP, a use case or a handler from its contract with the regenerable part, the owned part with one hole and one red test per method, the shared support package and the wiring, because the agent should decide domain logic only.",
		"M7 adds golaw gen and golaw gen --check, which regenerate every regenerable file from its contracts and fail the pipeline on drift or an open hole, because a hand edit of generated code and a forgotten hole must never reach the gate.",
		"M7 adds golaw map, a repository map within 2 000 tokens with the open holes first, given to the agent at session start, because exploring five adapters to learn a pattern costs more than reading one line.",
		"M7 regenerates the kinds of an edited contract in the post hook and returns only the holes and the decisions, inside the hook budgets, because the agent should never have to remember the generator.",
		"M7 pins sqlc v1.31.1, oapi-codegen v2.8.0 and goverter v1.11.1 in the tools module of an enrolled repository and moves their output under gen and sqlcgen directories, because the kit's fixed outbound path caused a layout drift and the pipeline excludes those names.",
		"M7 counts golaw generated files in both coverage commands and adds revive argument-limit 7 to the lint asset, because the local number must not be greener than the gate and go:S107 had no local rule.",
		"M7 records its acceptance rows in docs/acceptance/m7.md and the kinds, the holes and the template laws in docs/generate.md, because a green by construction claim names the law and the test that holds it.",
	}
}

func TestM7DatedChangeNotes(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "CHANGELOG.md"))
	notes := m7Notes()
	expressions := make([]*regexp.Regexp, 0, len(notes))
	for _, note := range notes {
		expressions = append(expressions, regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: `+regexp.QuoteMeta(note)+`$`))
	}
	forbidden := []string{"golaw calls the Sonar", "M7 proves the Sonar way gate"}
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-date", regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: (M7 )`).ReplaceAllString(text, "- $1"), false},
		{"red-missing-reason", strings.Replace(text, "because the agent should decide domain logic only.", "", 1), false},
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
