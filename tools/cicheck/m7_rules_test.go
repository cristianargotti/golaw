package cicheck_test

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

const (
	m7RulesPath = "docs/rules.md"
	m7GenRule   = "rules/4.17@1"
)

var errM7Rules = errors.New("M7 rules section")

// m7RuleStatements are the sentences of the gen section and of the two
// paragraphs M7 amends that a reader relies on.
func m7RuleStatements() []string {
	return []string{
		"## " + m7GenRule,
		"The gen family keeps generated files in the hands of their generators",
		"a person is never denied",
		"golaw gen --check fails the pipeline on a hand edit, a missing regenerable file, a stale third party output or an open hole",
		"| Check | Rule | Needs | Why and boundary |",
		"the gen family keeps agents on the contract",
		"a golaw regenerable file, whose header names golaw and its kind, is held to both limits like hand written code",
	}
}

func m7RulesFit(text string) error {
	for _, statement := range m7RuleStatements() {
		if !strings.Contains(text, statement) {
			return fmt.Errorf("%w: docs/rules.md lacks %q", errM7Rules, statement)
		}
	}
	_, after, _ := strings.Cut(text, "## "+m7GenRule)
	section, _, _ := strings.Cut(after, "\n## Repository laws")
	if strings.Contains(section, "rules/4.16@1 |") {
		return fmt.Errorf("%w: the gen section holds a row of another rule", errM7Rules)
	}
	return nil
}

func TestM7GenRuleSection(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m7RulesPath))
	rows := []struct {
		name, text string
		bad        bool
	}{
		{"repository", text, false},
		{"red-missing-person", strings.Replace(text, "a person is never denied", "", 1), true},
		{"red-generated-exempt", strings.Replace(text, "is held to both limits like hand written code", "is exempt", 1), true},
		{"near-a-row-of-the-family", strings.Replace(text, "| Check | Rule | Needs | Why and boundary |\n| --- | --- | --- | --- |\n",
			"| Check | Rule | Needs | Why and boundary |\n| --- | --- | --- | --- |\n| gen.sample | rules/4.17@1 | none | A sample, because a row carries its why. |\n", 1), false},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if strings.Contains(row.text, "## "+m7GenRule) {
				if err := m7RulesFit(row.text); (err != nil) != row.bad {
					t.Fatalf("rules error = %v, want error %v", err, row.bad)
				}
				return
			}
			if !row.bad {
				t.Fatal("the gen section is missing")
			}
		})
	}
}

func TestM7ChangelogDatesTheGenRule(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, "CHANGELOG.md"))
	line := regexp.MustCompile(`(?m)^- \d{4}-\d{2}-\d{2}: the gen family, rule rules/4\.17@1, .*, because .*\.$`)
	if !m4ChangeNotesOK(text, []*regexp.Regexp{line}, []string{"golaw calls the Sonar"}) {
		t.Fatal("CHANGELOG.md dates no line of the gen family under rules/4.17@1")
	}
	if m4ChangeNotesOK(strings.Replace(text, "the gen family, rule rules/4.17@1", "the gen family", 1), []*regexp.Regexp{line}, nil) {
		t.Fatal("a line without the rule reference passed")
	}
}
