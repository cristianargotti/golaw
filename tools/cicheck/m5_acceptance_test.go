package cicheck_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	m5RecordPath  = "docs/acceptance/m5.md"
	m5RemoteGate  = "docs/live/remote-gate.json"
	m5Limitation  = "No remote rejection is recorded: the generic limitation of docs/profiles.md holds, and the kit claims no remote hard gate."
	m5Demonstrate = "A remote rejection is recorded in docs/live/remote-gate.json"
)

// errM5Record marks a record that does not fit the required rows.
var errM5Record = errors.New("M5 acceptance record")

// m5AcceptanceRows are the required rows of contract section 2.8, by
// category, in the order the report prints them.
func m5AcceptanceRows() map[string][]string {
	return map[string][]string{
		"m5-profile":  {"fixture-fails-with-profile", "fixture-passes-without", "fixture-passes-corrected", "digest-mismatch", "schema-mismatch", "stale-bundle"},
		"m5-install":  {"install", "install-idempotent", "install-mirror", "refresh", "rollback", "remove", "pin-unchanged"},
		"m5-adoption": {"init-profile-pins", "init-profile-refresh", "template-born-no-github-write", "init-profile-ci-line", "debt-seeded-once", "debt-broadening-refused"},
		"m5-fresh":    {"scan", "scan-enrolled", "rung", "hook-enrolled", "init-apply", "agent-install", "unenrolled-allow"},
		"m5-release":  {"dry-run-refused", "dry-run-accepted", "doctor-verified", "doctor-tampered-binary", "doctor-checksums-mismatch", "doctor-wrong-identity", "doctor-missing-attestation", "doctor-go-install", "doctor-source-build"},
		"m5-remote":   {"gate"},
	}
}

// m5Cell accepts what a native run can retain for one system: a status of
// the report, or the statement that no native run exists.
func m5Cell(cell string) bool {
	for _, status := range []string{"PASS", "FAIL", "not run"} {
		if strings.HasPrefix(cell, status) {
			return true
		}
	}
	return false
}

// m5RecordFits judges the record: its fixed statements, one row per
// required row with a cell per system, and the outcome of the remote gate
// that agrees with the presence of the owner's record.
func m5RecordFits(text string, recorded bool) error {
	for _, statement := range []string{
		"# M5 acceptance", "go run ./tools/acceptance -m5", "Candidate version", "Candidate binary SHA-256",
		"Execution location", "| Row | Linux | macOS | Windows |",
		"Linux and Windows rows count only from a native run of the m5 job, never from macOS or a cross build.",
		"SKIP never counts as PASS.", "Remote CI or the platform pipeline is the only hard gate.",
	} {
		if !strings.Contains(text, statement) {
			return fmt.Errorf("%w: the record lacks %q", errM5Record, statement)
		}
	}
	for category, names := range m5AcceptanceRows() {
		for _, name := range names {
			if err := m5RecordRow(text, category+"/"+name); err != nil {
				return err
			}
		}
	}
	if strings.Contains(text, m5Limitation) == recorded || strings.Contains(text, m5Demonstrate) != recorded {
		return fmt.Errorf("%w: the remote gate outcome disagrees with %s present=%v", errM5Record, m5RemoteGate, recorded)
	}
	return nil
}

// m5RecordRow requires the row once, with a retained cell per system.
func m5RecordRow(text, row string) error {
	prefix := "| " + row + " |"
	if strings.Count(text, prefix) != 1 {
		return fmt.Errorf("%w: row %s must occur once", errM5Record, row)
	}
	_, line, _ := strings.Cut(text, prefix)
	line, _, _ = strings.Cut(line, "\n")
	cells := strings.Split(strings.TrimSuffix(line, "|"), "|")
	if len(cells) != 3 {
		return fmt.Errorf("%w: row %s needs a Linux, a macOS and a Windows cell", errM5Record, row)
	}
	for _, cell := range cells {
		if !m5Cell(strings.TrimSpace(cell)) {
			return fmt.Errorf("%w: row %s holds %q, neither a status nor not run", errM5Record, row, strings.TrimSpace(cell))
		}
	}
	return nil
}

func TestM5AcceptanceRecord(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m5RecordPath))
	_, err := os.Stat(filepath.Join("..", "..", filepath.FromSlash(m5RemoteGate)))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	recorded := err == nil
	row := "| m5-fresh/rung |"
	rows := []struct {
		name, text string
		recorded   bool
		bad        bool
	}{
		{"repository", text, recorded, false},
		{"red-missing-row", strings.Replace(text, row, "| m5-fresh/rungs |", 1), recorded, true},
		{"red-skip-cell", strings.Replace(text, row+" not run |", row+" SKIP |", 1), recorded, true},
		{"red-missing-native-limit", strings.Replace(text, "never from macOS or a cross build.", "", 1), recorded, true},
		{"red-outcome-disagrees", text, !recorded, true},
		{"near-pass-with-evidence", strings.Replace(text, row+" not run |", row+" PASS, run 12 of the m5 job |", 1), recorded, false},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := m5RecordFits(row.text, row.recorded); (err != nil) != row.bad {
				t.Fatalf("record error = %v, want error %v", err, row.bad)
			}
		})
	}
}
