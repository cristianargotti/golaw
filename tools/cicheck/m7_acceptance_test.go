package cicheck_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
)

const m7RecordPath = "docs/acceptance/m7.md"

var errM7Record = errors.New("M7 acceptance record")

// m7AcceptanceRows are the required rows of the M7 categories, in the
// order the report prints them. The agent rows are named through the
// registered agent ids, the way m7InstructionPaths names their files,
// because the public tree outside the agent paths names no tool.
func m7AcceptanceRows() map[string][]string {
	ids := agents.IDs()
	first, second, third := ids[0], ids[1], ids[2]
	return map[string][]string{
		"m7-fresh":  {"ci-green", "new-code", "duplication", "holes-open", "rendering-digest"},
		"m7-check":  {"clean-after-fill", "drift-edited", "drift-missing", "drift-third-party", "open-hole"},
		"m7-denial": {first + "-write", first + "-edit", first + "-redirect", second + "-update", second + "-delete", second + "-redirect", third + "-feedback", "third-party", "header-stripped", "manifest-write", "handwritten-bare", "near-misses"},
		"m7-regen":  {first + "-write", first + "-edit", second + "-apply-patch", first + "-bash-heredoc", third + "-post", "regenerate-off", "port-p95", "query-p95", "openapi-p95", "slow-generator-killed"},
		"m7-layout": {"detected-layout", "ambiguous-layout"},
		"m7-map":    {"budget", "holes-first"},
		"m7-golden": {"kinds"},
	}
}

func m7Cell(cell string) bool {
	if rest, found := strings.CutPrefix(cell, "SKIP, "); found {
		return rest != ""
	}
	for _, status := range []string{"PASS", "FAIL", "not run"} {
		if strings.HasPrefix(cell, status) {
			return true
		}
	}
	return false
}

func m7RowFits(text, category, name string) error {
	prefix := "| " + category + "/" + name + " |"
	if strings.Count(text, prefix) != 1 {
		return fmt.Errorf("%w: row %s/%s must occur once", errM7Record, category, name)
	}
	_, line, _ := strings.Cut(text, prefix)
	line, _, _ = strings.Cut(line, "\n")
	cells := strings.Split(strings.TrimSuffix(line, "|"), "|")
	if len(cells) != 3 {
		return fmt.Errorf("%w: row %s/%s needs a Linux, a macOS and a Windows cell", errM7Record, category, name)
	}
	for _, cell := range cells {
		if !m7Cell(strings.TrimSpace(cell)) {
			return fmt.Errorf("%w: row %s/%s holds %q", errM7Record, category, name, strings.TrimSpace(cell))
		}
	}
	return nil
}

func m7RecordFits(text string) error {
	for _, statement := range []string{
		"# M7 acceptance", "go run ./tools/acceptance -m7", "Candidate version", "Candidate binary SHA-256",
		"Execution location", "| Row | Linux | macOS | Windows |",
		"Linux and Windows rows count only from a native run, never from macOS or a cross build.",
		"SKIP never counts as PASS.", "Remote CI or the platform pipeline is the only hard gate.",
		"the three native cells of m7-fresh/rendering-digest record one digest",
		"no row of this record claims it",
	} {
		if !strings.Contains(text, statement) {
			return fmt.Errorf("%w: the record lacks %q", errM7Record, statement)
		}
	}
	for category, names := range m7AcceptanceRows() {
		for _, name := range names {
			if err := m7RowFits(text, category, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestM7AcceptanceRecord(t *testing.T) {
	t.Parallel()
	if len(agents.IDs()) != 3 {
		t.Fatal("expected three registered agents")
	}
	text := string(kitBytes(t, m7RecordPath))
	row := "| m7-denial/" + agents.IDs()[0] + "-write |"
	rows := []struct {
		name, text string
		bad        bool
	}{
		{"repository", text, false},
		{"red-missing-row", strings.Replace(text, row, "| m7-denial/"+agents.IDs()[0]+"-writes |", 1), true},
		{"red-bare-skip-cell", strings.Replace(text, row+" not run |", row+" SKIP |", 1), true},
		{"red-live-claimed", strings.Replace(text, "no row of this record claims it", "", 1), true},
		{"red-identity-unstated", strings.Replace(text, "record one digest", "", 1), true},
		{"near-skip-with-cause", strings.Replace(text, row+" not run |", row+" SKIP, sqlc v1.31.1 did not build |", 1), false},
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := m7RecordFits(tc.text); (err != nil) != tc.bad {
				t.Fatalf("record error = %v, want error %v", err, tc.bad)
			}
		})
	}
}
