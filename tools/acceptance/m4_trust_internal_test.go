package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/install"
)

func TestM4TrustRecordReproducesTheSurveyHash(t *testing.T) {
	t.Parallel()
	hooks := "/opt/codex/hooks.json"
	entries := []m4Entry{{
		Event: eventPre, Matcher: "^(Bash|apply_patch)$",
		Raw: json.RawMessage(`{"type":"command","command":"'/usr/local/bin/golaw' hook codex","timeout":10}`),
	}}
	record, err := m4TrustRecord(hooks, entries, "linux")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), codexConfig)
	checkedWrite(t, path, record)
	trusted, err := install.CodexTrusted(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "sha256:8a558a899c13b8c1879f497999eb9d26aa178f642737d2ed40bdae925cb441e3"
	if got := trusted[hooks+":pre_tool_use:0:0"]; got != want || len(trusted) != 1 {
		t.Fatalf("record %q reads back as %v", record, trusted)
	}
	if _, err := m4TrustRecord(hooks, nil, "linux"); err == nil {
		t.Fatal("a record of no handler passed")
	}
}

// m4TrustDoctor answers doctor --json as doctor-trust answers it from the
// record: trusted when config.toml holds the hash of every golaw handler,
// revalidation required when it holds another, untrusted without it, after
// edit.
func m4TrustDoctor(t *testing.T, s *suite, c command, edit func(*doctor.Line)) result {
	t.Helper()
	home := environmentValue(c.env, "HOME")
	path := m4HooksPath(home, agentCodex)
	entries, err := m4ReadEntries(path, agentCodex)
	if err != nil {
		t.Fatal(err)
	}
	current, err := m4TrustRecord(path, m4GolawEntries(entries, agentCodex, c.program, s.m4System()), s.m4System())
	if err != nil {
		t.Fatal(err)
	}
	line := doctor.Line{ID: lineTrusted, Key: "codex/user", State: "red", Code: "untrusted"}
	stored, err := os.ReadFile(filepath.Join(home, codexDir, codexConfig))
	switch {
	case err == nil && string(stored) == current:
		line.State, line.Code = "ok", "trusted"
	case err == nil:
		line.Code = "revalidation-required"
	case !errors.Is(err, fs.ErrNotExist):
		t.Fatal(err)
	}
	if edit != nil {
		edit(&line)
	}
	codexReport, err := json.Marshal(doctor.Report{Schema: 1, Lines: []doctor.Line{line}})
	if err != nil {
		t.Fatal(err)
	}
	empty := `{"schema":1,"lines":[]}`
	return result{stdout: empty + "\n" + string(codexReport) + "\n" + empty + "\n"}
}

// m4TrustSuite is a Linux suite whose installer writes golaw's hooks under
// the home of the command and whose doctor answers through m4TrustDoctor.
func m4TrustSuite(t *testing.T, edit func(call int, line *doctor.Line)) *suite {
	t.Helper()
	s := testSuite(t)
	s.m2OS = "linux"
	base, calls := s.execute, 0
	s.execute = func(ctx context.Context, c command) result {
		switch {
		case len(c.args) == 4 && c.args[0] == "agent":
			m4Installed(t, environmentValue(c.env, "HOME"), c.program, s.m4System())
			return result{}
		case len(c.args) == 2 && c.args[0] == "doctor":
			calls++
			call := calls
			return m4TrustDoctor(t, s, c, func(line *doctor.Line) {
				if edit != nil {
					edit(call, line)
				}
			})
		}
		return base(ctx, c)
	}
	return s
}

func TestM4TrustRowReportsTheChangedDefinition(t *testing.T) {
	t.Parallel()
	s := m4TrustSuite(t, nil)
	rows := s.m4TrustRows(t.Context(), false)
	if len(rows) != 1 || rows[0].status != statusPass || rows[0].name != m4ChangedDefinition {
		t.Fatalf("rows = %+v", rows)
	}
	for _, tc := range []struct {
		name, want string
		edit       func(int, *doctor.Line)
	}{
		{"red a doctor that still trusts the changed handler", "step 5", func(call int, line *doctor.Line) {
			if call == 2 {
				line.State, line.Code = "ok", "trusted"
			}
		}},
		{"red a doctor that never reads the record", "step 3", func(_ int, line *doctor.Line) { line.State, line.Code = "red", "untrusted" }},
		{"red a report without the trust line", "lacks doctor.trusted", func(_ int, line *doctor.Line) { line.ID = "doctor.other" }},
	} {
		s := m4TrustSuite(t, tc.edit)
		if got := s.m4TrustRows(t.Context(), false)[0]; got.status != statusFail || !strings.Contains(got.detail, tc.want) {
			t.Errorf("%s: %+v", tc.name, got)
		}
	}
}
