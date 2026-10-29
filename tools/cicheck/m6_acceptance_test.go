package cicheck_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const m6RecordPath = "docs/acceptance/m6.md"

// errM6Record marks a record that does not fit the required rows.
var errM6Record = errors.New("M6 acceptance record")

// m6AcceptanceRows are the required rows of the M6 categories, in the
// order the report prints them.
func m6AcceptanceRows() map[string][]string {
	return map[string][]string{
		"m6-surface":  {"red-scan", "red-lint", "corrected-scan", "corrected-lint"},
		"m6-missing":  {"doctor-lines", "hook-notice", "rung-notice", "lint-unmeasured"},
		"m6-budget":   {"post-p95", "pre-p95", "slow-tool-killed"},
		"m6-coverage": {"parity-number", "strict-differs", "missing-report", "stale-report"},
		"m6-denial":   {"code-eval", "hardcoded-credential", "file-length", "inline-script-size", "suppression", "analyzer-widening", "tag-and-digest", "corrected-twin"},
		"m6-setup":    {"tampered-asset"},
		"m6-config":   {"repository-config-ignored"},
	}
}

// m6Cell accepts what a run can retain for one system: a status of the
// report, a SKIP that names its cause, or the statement that no native
// run exists.
func m6Cell(cell string) bool {
	if strings.HasPrefix(cell, "SKIP, ") && strings.TrimPrefix(cell, "SKIP, ") != "" {
		return true
	}
	for _, status := range []string{"PASS", "FAIL", "not run"} {
		if strings.HasPrefix(cell, status) {
			return true
		}
	}
	return false
}

func m6RecordFits(text string) error {
	for _, statement := range []string{
		"# M6 acceptance", "go run ./tools/acceptance -m6", "Candidate version", "Candidate binary SHA-256",
		"Execution location", "| Row | Linux | macOS | Windows |",
		"Linux and Windows rows count only from a native run, never from macOS or a cross build.",
		"SKIP never counts as PASS.", "Remote CI or the platform pipeline is the only hard gate.",
		"no row of this record claims it",
	} {
		if !strings.Contains(text, statement) {
			return fmt.Errorf("%w: the record lacks %q", errM6Record, statement)
		}
	}
	for category, names := range m6AcceptanceRows() {
		for _, name := range names {
			if err := m6RowFits(text, category, name); err != nil {
				return err
			}
		}
	}
	return nil
}

// m6RowFits holds one required row to a single line with three valid cells.
func m6RowFits(text, category, name string) error {
	prefix := "| " + category + "/" + name + " |"
	if strings.Count(text, prefix) != 1 {
		return fmt.Errorf("%w: row %s/%s must occur once", errM6Record, category, name)
	}
	_, line, _ := strings.Cut(text, prefix)
	line, _, _ = strings.Cut(line, "\n")
	cells := strings.Split(strings.TrimSuffix(line, "|"), "|")
	if len(cells) != 3 {
		return fmt.Errorf("%w: row %s/%s needs a Linux, a macOS and a Windows cell", errM6Record, category, name)
	}
	for _, cell := range cells {
		if !m6Cell(strings.TrimSpace(cell)) {
			return fmt.Errorf("%w: row %s/%s holds %q", errM6Record, category, name, strings.TrimSpace(cell))
		}
	}
	return nil
}

func TestM6AcceptanceRecord(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m6RecordPath))
	row := "| m6-denial/code-eval |"
	rows := []struct {
		name, text string
		bad        bool
	}{
		{"repository", text, false},
		{"red-missing-row", strings.Replace(text, row, "| m6-denial/code-evals |", 1), true},
		{"red-bare-skip-cell", strings.Replace(text, row+" not run |", row+" SKIP |", 1), true},
		{"red-missing-native-limit", strings.Replace(text, "never from macOS or a cross build.", "", 1), true},
		{"red-harness-claimed", strings.Replace(text, "no row of this record claims it", "", 1), true},
		{"near-skip-with-cause", strings.Replace(text, row+" not run |", row+" SKIP, node is not on PATH |", 1), false},
		{"near-pass-with-evidence", strings.Replace(text, row+" not run |", row+" PASS, run 12 of a native job |", 1), false},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := m6RecordFits(tc.text); (err != nil) != tc.bad {
				t.Fatalf("record error = %v, want error %v", err, tc.bad)
			}
		})
	}
}
