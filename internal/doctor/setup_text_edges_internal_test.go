package doctor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runnertest"
)

func TestSetupRoutingDetailsPreservePlainPaths(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, local string
	}{
		{"unix", "/repo with spaces/.git/hooks", ""},
		{"windows", `C:\work\app\.git\hooks`, ""},
		{"local", "/repo/custom-hooks", "custom-hooks"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts := setupBaseline()
			facts.Receipt.HooksPath, facts.EffectiveHooksPath = tc.path, tc.path
			facts.LocalHooksPath = tc.local
			report := Report{Schema: 1, Lines: setupRoutingLines(facts)}
			var out bytes.Buffer
			if err := Print(&out, report, true); err != nil {
				t.Fatal(err)
			}
			var replay Report
			if err := json.Unmarshal(out.Bytes(), &replay); err != nil {
				t.Fatal(err)
			}
			local := tc.local
			if local == "" {
				local = "unset"
			}
			want := "effective hooks path " + tc.path + "; local core.hooksPath " + local
			if got := replay.Lines[0].Detail; got != want {
				t.Fatalf("routing detail = %q, want %q", got, want)
			}
		})
	}
}

func TestSetupRoutingFailuresStayOnOneTextLine(t *testing.T) {
	bin := t.TempDir()
	runnertest.Install(t, bin, "git", runnertest.Behavior{
		Stderr: "configuration unreadable\ncheck permissions\n", Code: 2,
	})
	t.Setenv("PATH", bin)
	facts := setupBaseline()
	collectSetupRouting(t.Context(), t.TempDir(), nil, &facts)
	if strings.ContainsAny(facts.HooksPathError, "\r\n") {
		t.Fatalf("routing evidence spans lines: %q", facts.HooksPathError)
	}
	for _, query := range []string{"config --local --get core.hooksPath", "rev-parse --path-format=absolute --git-path hooks"} {
		if !strings.Contains(facts.HooksPathError, query) {
			t.Fatalf("routing query lost: %s in %s", query, facts.HooksPathError)
		}
	}
	var out bytes.Buffer
	if err := Print(&out, Report{Schema: 1, Lines: setupRoutingLines(facts)}, false); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "\n") != 2 {
		t.Fatalf("routing report spans lines: %s", out.String())
	}
}

//nolint:paralleltest // The collected fixtures replace PATH through collectRow.
func TestSetupTextReportsCollectedEvidence(t *testing.T) {
	for _, tc := range []collectCase{
		{name: "armed", receipt: "present", version: "8.30.1"},
		{name: "missing tool", receipt: "present", missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newCollectFixture(t, tc)
			facts := collectRow(t, f, tc)
			report := Report{Schema: 1, Lines: AssessSetup(facts)}
			var out bytes.Buffer
			if err := Print(&out, report, false); err != nil {
				t.Fatal(err)
			}
			if strings.Count(out.String(), "\n") != len(report.Lines)+1 {
				t.Fatalf("diagnostic escaped its text line: %s", out.String())
			}
			t.Logf("collected text report:\n%s", out.String())
		})
	}
}
