package doctor

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
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
	hooks := []string{"doctor.setup-hooks ok", "doctor.setup-hooks ok", "doctor.setup-hooks ok"}
	for _, tc := range []struct {
		row  collectCase
		want []string
	}{
		{collectCase{name: "armed", receipt: "present", version: "8.30.1"}, slices.Concat(
			[]string{"doctor.setup-receipt ok"}, hooks, []string{"doctor.setup-routing ok"},
			setupToolStates("gitleaks ok"),
		)},
		// Without git on PATH the routing is unknown and the rest stays.
		{collectCase{name: "missing tool", receipt: "present", missing: true}, slices.Concat(
			[]string{"doctor.setup-receipt ok"}, hooks, []string{"doctor.setup-routing unknown"},
			setupToolStates("gitleaks missing"),
		)},
	} {
		t.Run(tc.row.name, func(t *testing.T) {
			f := newCollectFixture(t, tc.row)
			report := Report{Schema: 1, Lines: AssessSetup(collectRow(t, f, tc.row))}
			if got := setupLineStates(report.Lines); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("collected diagnostics = %v, want %v", got, tc.want)
			}
			var out bytes.Buffer
			if err := Print(&out, report, false); err != nil {
				t.Fatal(err)
			}
			if strings.Count(out.String(), "\n") != len(report.Lines)+1 {
				t.Fatalf("diagnostic escaped its text line: %s", out.String())
			}
		})
	}
}

// setupLineStates is the id and state of each line, with the key of a tool
// line, whose key names the tool where the other keys are paths of a run.
func setupLineStates(lines []Line) []string {
	states := make([]string, 0, len(lines))
	for _, line := range lines {
		if line.ID == "doctor.setup-tools" {
			states = append(states, line.ID+" "+line.Key+" "+line.State)
			continue
		}
		states = append(states, line.ID+" "+line.State)
	}
	return states
}

// setupToolStates is one tool line per pinned tool, in toolchain order: the
// gitleaks state of a row and every other tool ok.
func setupToolStates(gitleaks string) []string {
	states := make([]string, 0, len(runner.Toolchain()))
	for _, tool := range runner.Toolchain() {
		state := tool.Name + " ok"
		if tool.Name == "gitleaks" {
			state = gitleaks
		}
		states = append(states, "doctor.setup-tools "+state)
	}
	return states
}
