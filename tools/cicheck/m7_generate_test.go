package cicheck_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/goldentest"
	"github.com/cristianargotti/golaw/internal/gen/kinds"
)

const (
	m7GeneratePath = "docs/generate.md"
	m7KindHeader   = "| Kind | Template | Command | Regenerable part | Owned part |"
	m7LawHeader    = "| Law | Gate condition | What the templates never emit |"
	m7RuleHeader   = "| Rule or linter | Shape the sessions had to rewrite | What the templates do instead | Proof in the kit |"
)

var errM7Generate = errors.New("docs/generate.md")

func m7GenerateStatements() []string {
	return []string{
		"# Generators",
		"Every check on the developer's machine is optional. The pipeline or remote CI is the only gate.",
		"golaw never calls a Sonar server, a scanner or its API",
		"The engine never rewrites or deletes owned code",
		"A person is never denied",
		"goverter never runs in a hook",
		"killed at 1.5 s, and the external work of one event stops at 3 s",
		"`ladder.hook_regenerate: false` turns the regeneration off without a finding",
		"golaw setup --gen",
		"## The gaps",
		m7SkillRule,
	}
}

// m7RowsStart holds every want to one row of the table under header that
// starts with it.
func m7RowsStart(text, header string, wants []string) error {
	rows := m6TableRows(text, header)
	for _, want := range wants {
		n := 0
		for _, row := range rows {
			if strings.HasPrefix(row, want) {
				n++
			}
		}
		if n != 1 {
			return fmt.Errorf("%w: %d rows start %q under %q", errM7Generate, n, want, header)
		}
	}
	return nil
}

func m7GenerateFits(text string) error {
	for _, statement := range m7GenerateStatements() {
		if !strings.Contains(text, statement) {
			return fmt.Errorf("%w: lacks %q", errM7Generate, statement)
		}
	}
	var kindRows, lawRows []string
	for _, k := range kinds.All() {
		kindRows = append(kindRows, fmt.Sprintf("| %s | %s@%d |", k.Name(), k.Name(), k.Template()))
	}
	for _, law := range goldentest.Laws() {
		lawRows = append(lawRows, "| "+law.Name+" |")
	}
	if err := m7RowsStart(text, m7KindHeader, kindRows); err != nil {
		return err
	}
	if err := m7RowsStart(text, m7LawHeader, lawRows); err != nil {
		return err
	}
	return m7RowsStart(text, m7RuleHeader, []string{"| go:S107", "| go:S3776", "| go:S4144", "| go:S1192", "| duplication over 3 percent", "| coverage on new code"})
}

func TestM7GenerateGuide(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m7GeneratePath))
	rows := []struct {
		name, text string
		bad        bool
	}{
		{"repository", text, false},
		{"red-missing-kind", strings.Replace(text, "| adapter-sql | adapter-sql@1 |", "| adapter sql | adapter-sql@1 |", 1), true},
		{"red-missing-law", strings.Replace(text, "| no-regexp |", "| no regexp |", 1), true},
		{"red-optional-gate", strings.Replace(text, "The pipeline or remote CI is the only gate.", "", 1), true},
		{"red-skill-rule-reworded", strings.Replace(text, "must pass before you finish", "should pass", 1), true},
		{"near-appended-note", text + "\nA note a later release adds.\n", false},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := m7GenerateFits(row.text); (err != nil) != row.bad {
				t.Fatalf("guide error = %v, want error %v", err, row.bad)
			}
		})
	}
}

// TestM7GuideQuotesTheSkill holds the guide and both copies of the skill to
// one rule, so the agent and the reader never meet two versions.
func TestM7GuideQuotesTheSkill(t *testing.T) {
	t.Parallel()
	skills, _ := m7InstructionPaths(t)
	for _, path := range append([]string{m7GeneratePath}, skills...) {
		text := string(kitBytes(t, path))
		if !strings.Contains(text, m7SkillRule) {
			t.Errorf("%s lacks the rule of M7.9.5", path)
		}
	}
}
