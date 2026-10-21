package cicheck_test

import (
	"strings"
	"testing"
)

const (
	m3ChecklistsPath = "docs/checklists.md"
	m3ContextPath    = "docs/context-template.md"
)

func TestM3EngineeringGuides(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name string
		path string
		text string
		want bool
	}{
		{"builder", m3ChecklistsPath, "## Builder", true},
		{"reviewer", m3ChecklistsPath, "## Reviewer", true},
		{"conditions", m3ChecklistsPath, "## Conditional checks", true},
		{"evidence", m3ChecklistsPath, "## Evidence", true},
		{"near local choice", m3ChecklistsPath, "Every local check is optional.", true},
		{"near remote evidence", m3ChecklistsPath, "Remote CI or the platform pipeline is the only gate.", true},
		{"red compulsory setup", m3ChecklistsPath, "Every developer must run golaw setup.", false},
		{"red heavy push", m3ChecklistsPath, "Run mutation testing in pre-push.", false},
		{"measurement", m3ChecklistsPath, "A check needs a red case, a near miss and an explicit input universe.", true},
		{"process results", m3ChecklistsPath, "Read both the exit code and stderr.", true},
		{"wall clock", m3ChecklistsPath, "Use benchmarks for wall-clock budgets.", true},
		{"equivalence", m3ChecklistsPath, "An equivalence claim names the applied input and identical observed result.", true},
		{"query plans", m3ChecklistsPath, "generate_series, ANALYZE and EXPLAIN", true},
		{"architecture evidence", m3ChecklistsPath, "GOARCH=amd64", true},
		{"one heavy process", m3ChecklistsPath, "Run only one heavy process at a time on the workstation.", true},
		{"transactions", m3ChecklistsPath, "Keep network calls outside transactions.", true},
		{"atomic money", m3ChecklistsPath, "Commit a money-path change as one database unit.", true},
		{"retry limits", m3ChecklistsPath, "Bound retries, provide a parking path, back off and expose permanent failure through liveness.", true},
		{"price constraints", m3ChecklistsPath, "Price constraints use CHECK, then NOT VALID and VALIDATE when expanding an existing table.", true},
		{"fingerprints", m3ChecklistsPath, "Store a version beside every persisted fingerprint.", true},
		{"retention", m3ChecklistsPath, "Every finite retention policy has a purge query.", true},
		{"database budgets", m3ChecklistsPath, "statement_timeout, lock_timeout and idle_in_transaction_session_timeout", true},
		{"idempotency", m3ChecklistsPath, "Test simultaneous duplicate delivery and the partial unique index.", true},
		{"contract identity", m3ChecklistsPath, "Expose the contract hash in liveness.", true},
		{"log identity", m3ChecklistsPath, "Use one version per process and log service, env, version and trace identifiers.", true},
		{"histograms", m3ChecklistsPath, "The highest finite histogram bucket covers the configured ceiling.", true},
		{"deadline budget", m3ChecklistsPath, "Budget deadlines from the request boundary to the database.", true},
		{"foreign conventions", m3ChecklistsPath, "Read the destination repository's branch, title and body conventions.", true},
		{"squash message", m3ChecklistsPath, "Check the author address and final squash message for an added trailer.", true},
		{"managed objects", m3ChecklistsPath, "Each managed object names its property, count, policy family, alternatives and reversal condition.", true},
		{"tool agreement", m3ChecklistsPath, "Two tools must not disagree silently.", true},
		{"recurrence", m3ChecklistsPath, "A repeated finding becomes a check when a meaningful red case and near miss exist.", true},
		{"lesson", m3ChecklistsPath, "Record a lesson the same day and date its change with a reason.", true},
		{"review ownership", m3ChecklistsPath, "Use tools for verification and one reviewer for judgment.", true},
		{"context boundaries", m3ContextPath, "## Boundaries", true},
		{"context shapes", m3ContextPath, "## Shapes", true},
		{"context tests", m3ContextPath, "## Tests", true},
		{"context order", m3ContextPath, "## Construction order", true},
		{"plain structs", m3ContextPath, "Use plain application input and output structs.", true},
		{"repository mapping", m3ContextPath, "Rows map to domain types in the repository.", true},
		{"handler signature", m3ContextPath, "Keep the generated strict handler signature.", true},
		{"clock", m3ContextPath, "Inject a Clock port and compose UTC at the entry point.", true},
		{"configuration", m3ContextPath, "Each configurable value has an env tag, a default, validation and documentation.", true},
		{"near no scaffolding", m3ContextPath, "This guide does not cause init to scaffold application source.", true},
	}
	for _, row := range rows {
		got := strings.Contains(string(kitBytes(t, row.path)), row.text)
		if got != row.want {
			t.Errorf("%s: content present=%v, want=%v", row.name, got, row.want)
		}
	}
}

func TestM3GuideLengths(t *testing.T) {
	t.Parallel()
	for _, path := range []string{m3ChecklistsPath, m3ContextPath} {
		if got := lineCount(kitBytes(t, path)); got > 300 {
			t.Errorf("%s has %d lines, maximum 300", path, got)
		}
	}
}
