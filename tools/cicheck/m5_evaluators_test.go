package cicheck_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks"
)

// errM5Evaluator marks a disagreement of the evaluator table with the catalog.
var errM5Evaluator = errors.New("profile evaluator table")

// m5Evaluators names every check that reads profile data with its rule.
func m5Evaluators() map[string]string {
	return map[string]string{
		"policy.image-stages":      m5ProfileRule,
		"policy.image-installs":    m5ProfileRule,
		"policy.image-registry":    m5ProfileRule,
		"policy.profile-targets":   m5ProfileRule,
		"policy.values-assertions": m5ProfileRule,
		"paths.profile-forbidden":  "rules/4.2@1",
		"text.tool-terms":          "rules/4.1@1",
	}
}

// m5Registered answers the needs of every registered check, as the guide
// writes them, and reports an evaluator registered under another rule.
func m5Registered(t *testing.T) map[string]string {
	t.Helper()
	needs := make(map[string]string)
	for _, check := range checks.Catalog() {
		spec := check.Spec()
		needs[spec.ID] = strings.Join(spec.Needs, ",")
		if needs[spec.ID] == "" {
			needs[spec.ID] = "none"
		}
		if rule, found := m5Evaluators()[spec.ID]; found && rule != spec.Rule {
			t.Errorf("%s registers rule %s, the guide's table holds %s", spec.ID, spec.Rule, rule)
		}
	}
	return needs
}

// m5EvaluatorRows judges the evaluator table of the profile guide against
// the registered needs: one row per evaluator, its needs as the spec states
// them, and no row for a check that is not an evaluator.
func m5EvaluatorRows(text string, needs map[string]string) error {
	rows := make(map[string]string)
	for line := range strings.SplitSeq(text, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 7 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "`") {
			continue
		}
		id := strings.Trim(strings.TrimSpace(cells[1]), "`")
		if _, found := rows[id]; found {
			return fmt.Errorf("%w: evaluator %s has two rows", errM5Evaluator, id)
		}
		rows[id] = strings.TrimSpace(cells[3])
	}
	for id := range m5Evaluators() {
		want, registered := needs[id]
		if !registered {
			return fmt.Errorf("%w: evaluator %s is not registered", errM5Evaluator, id)
		}
		if got, found := rows[id]; !found || got != want {
			return fmt.Errorf("%w: evaluator %s documents needs %q, want %q", errM5Evaluator, id, got, want)
		}
		delete(rows, id)
	}
	for id := range rows {
		return fmt.Errorf("%w: documented evaluator %s reads no profile data", errM5Evaluator, id)
	}
	return nil
}

func TestM5EvaluatorTable(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m5ProfilesPath))
	synthetic := map[string]string{
		"policy.image-stages": "image", "policy.image-installs": "image", "policy.image-registry": "image",
		"policy.profile-targets": "go", "policy.values-assertions": "none",
		"paths.profile-forbidden": "none", "text.tool-terms": "none",
	}
	unregistered := map[string]string{"policy.image-stages": "image"}
	registry := "| `policy.image-registry` | `registry.pattern` | image |"
	rows := []struct {
		name, text string
		needs      map[string]string
		bad        bool
	}{
		{"repository", text, m5Registered(t), false},
		{"near-synthetic-catalog", text, synthetic, false},
		{"red-missing-row", strings.Replace(text, registry, "| registry |", 1), synthetic, true},
		{"red-wrong-needs", strings.Replace(text, registry, "| `policy.image-registry` | `registry.pattern` | go |", 1), synthetic, true},
		{"red-unregistered", text, unregistered, true},
		{"red-foreign-row", text + "| `policy.makefile` | `targets` | go | `makefile` | none |\n", synthetic, true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := m5EvaluatorRows(row.text, row.needs); (err != nil) != row.bad {
				t.Fatalf("evaluator table error = %v, want error %v", err, row.bad)
			}
		})
	}
}

func TestM5EvaluatorLimits(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m5ProfilesPath))
	required := []string{
		"## Evaluators",
		"A profile needing new logic requires a generic core evaluator; it cannot execute bundle code.",
		"Each check answers NotApplicable without a profile and when the profile carries no section for it",
		"Only the root `Dockerfile` is judged",
		"an exec form install, a heredoc install, and an install behind an assignment, a shell keyword, a subshell, sudo, a path or an option that takes an argument before its verb are stated limits.",
		"A comment prefix compares without the mark, case and extra spaces.",
		"a trailing slash covers a tree, names compare without case, and a file under testdata is never judged.",
		"A parent that is not a mapping is a missing key",
		"keeps the kit's default terms.",
		"Unsupported syntax or unavailable external evidence is Unmeasured, never a text match.",
	}
	forbidden := []string{"These categories describe semantics, not serialized payload keys"}
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-missing-install-limit", strings.Replace(text, required[4], "", 1), false},
		{"red-category-table-kept", text + "\n" + forbidden[0] + ".\n", false},
		{"near-extra-note", text + "\nThe fixture bundle names no platform value.\n", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m4TextOK(row.text, required, forbidden); got != row.want {
				t.Fatalf("evaluator limits = %v, want %v", got, row.want)
			}
		})
	}
}
