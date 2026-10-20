package doctor

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/setup"
)

type setupProbeCase struct {
	name, stdout, stderr, version, state, reason string
	code                                         int
}

func TestSetupToolProbeFailuresRetainResolvedEvidence(t *testing.T) {
	tool, ok := runner.LookupTool("gitleaks")
	if !ok {
		t.Fatal("gitleaks is absent from the toolchain")
	}
	for _, tc := range []setupProbeCase{
		{"nonzero", "8.30.1\n", "probe refused", "8.30.1", "unknown", "exited 2: probe refused", 2},
		{
			"multiline", "8.30.1\nadditional output\n", "probe refused\nadditional error\n",
			"8.30.1", "unknown", "exited 2: probe refused", 2,
		},
		{"empty output", "", "", "", "unknown", "returned no version", 0},
		{"other version", "gitleaks version 8.30.0\n", "", "gitleaks version 8.30.0", "version", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := t.TempDir()
			path := runnertest.Install(t, bin, tool.Name, runnertest.Behavior{
				Probe: []string{"another-probe"}, Stdout: tc.stdout, Stderr: tc.stderr, Code: tc.code,
			})
			t.Setenv("PATH", bin)
			status := collectSetupTool(t.Context(), tool, nil, nil)
			requireSetupProbeEvidence(t, status, path, tc)
			if calls := runnertest.Calls(t, bin, tool.Name); len(calls) == 0 {
				t.Fatal("present tool was not probed")
			}
		})
	}
}

func requireSetupProbeEvidence(t *testing.T, status ToolStatus, path string, tc setupProbeCase) {
	t.Helper()
	if status.Path != path || status.Version != tc.version ||
		!strings.Contains(status.Error, tc.reason) {
		t.Fatalf("probe evidence = %+v", status)
	}
	if tc.reason == "" && status.Error != "" {
		t.Fatalf("successful other version became a probe failure: %+v", status)
	}
	line := setupToolLine(status)
	if line.State != tc.state || line.Code != "setup-tool-unavailable" {
		t.Fatalf("probe diagnosis = %+v", line)
	}
	if strings.ContainsAny(line.Detail, "\r\n") {
		t.Fatalf("probe diagnosis spans multiple lines: %q", line.Detail)
	}
}

func TestSetupToolCannotStartKeepsItsPath(t *testing.T) {
	t.Parallel()
	tool, ok := runner.LookupTool("gitleaks")
	if !ok {
		t.Fatal("gitleaks is absent from the toolchain")
	}
	path := filepath.Join(t.TempDir(), "missing")
	status := collectSetupProbe(t.Context(), tool, nil, ToolStatus{Name: tool.Name, Path: path})
	if status.Path != path || status.Error == "" || status.Version != "" {
		t.Fatalf("start failure = %+v", status)
	}
	if line := setupToolLine(status); line.State != "unknown" {
		t.Fatalf("start failure diagnosis = %+v", line)
	}
}

func TestSetupGovulncheckUsesReceiptVersionWithoutStarting(t *testing.T) {
	bin := t.TempDir()
	tool, ok := runner.LookupTool("govulncheck")
	if !ok {
		t.Fatal("govulncheck is absent from the toolchain")
	}
	path := runnertest.Install(t, bin, tool.Name, runnertest.Behavior{
		Probe: tool.Probe, Version: "99.0.0",
	})
	elsewhere := filepath.Join(t.TempDir(), filepath.Base(path))
	unclean := bin + string(filepath.Separator) + "." + string(filepath.Separator) + filepath.Base(path)
	for _, tc := range []struct {
		name, version, state, at string
		recorded                 bool
	}{
		{"previous pin", "1.6.0", "version", path, true},
		{"current pin", tool.Version, "ok", path, true},
		{"current pin at an unclean spelling of the path", tool.Version, "ok", unclean, true},
		{"empty recorded version", "", "version", path, true},
		{"absent record", "", "version", "", false},
		{"current pin recorded at another path", tool.Version, "unknown", elsewhere, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", bin)
			var recorded []setup.ToolRecord
			if tc.recorded {
				recorded = []setup.ToolRecord{{Name: tool.Name, Path: tc.at, Version: tc.version}}
			}
			status := collectSetupTool(t.Context(), tool, os.Environ(), recorded)
			requireSetupRecordedEvidence(t, status, path, tc.at, tc.version, tc.state)
			if calls := runnertest.Calls(t, bin, tool.Name); len(calls) != 0 {
				t.Fatalf("scanner was started: %v", calls)
			}
			requireSetupRecordedLine(t, status, tool, tc.state)
		})
	}
}

// requireSetupRecordedEvidence requires the path PATH resolved, with the
// version setup recorded only when setup recorded that same path: the
// version of another recorded path describes another binary, so that
// evidence keeps no version and its line names both paths.
func requireSetupRecordedEvidence(t *testing.T, status ToolStatus, path, at, version, state string) {
	t.Helper()
	if status.Path != path {
		t.Fatalf("recorded scanner evidence = %+v", status)
	}
	if state != "unknown" {
		if status.Version != version || status.Error != "" {
			t.Fatalf("recorded scanner evidence = %+v", status)
		}
		return
	}
	detail := setupToolLine(status).Detail
	if status.Version != "" || !strings.Contains(detail, path) || !strings.Contains(detail, at) {
		t.Fatalf("scanner recorded at %s, resolved at %s: %+v, detail %q", at, path, status, detail)
	}
}

func requireSetupRecordedLine(t *testing.T, status ToolStatus, tool runner.Tool, state string) {
	t.Helper()
	line := setupToolLine(status)
	if line.State != state {
		t.Fatalf("recorded provenance = %+v", line)
	}
	if state == "ok" {
		if !strings.Contains(line.Detail, "version recorded by setup; no probe") {
			t.Fatalf("recorded provenance = %+v", line)
		}
		return
	}
	requireSetupToolRemedy(t, line, tool)
	if status.Version != "" && !strings.Contains(line.Detail, status.Version) {
		t.Fatalf("recorded version is absent: %+v", line)
	}
}

func TestSetupToolAssessmentMatchesWholeVersionTokens(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		version, state string
	}{
		{"golangci-lint has version 2.14.0 built with go1.27.2\ncommit abc", "ok"},
		{"2.14.01", "version"},
		{"2.14.0-rc1", "version"},
	} {
		line := setupToolLine(ToolStatus{Name: "golangci-lint", Path: "bin/tool", Version: tc.version})
		if line.State != tc.state {
			t.Fatalf("version %q = %+v", tc.version, line)
		}
	}
	line := setupToolLine(ToolStatus{Name: "other-tool", Path: "bin/tool", Version: "2.14.0"})
	if line.State != "unknown" {
		t.Fatalf("unknown tool = %+v", line)
	}
}

func TestSetupToolMissingKeepsOnlyTheLookPathObservation(t *testing.T) {
	tool, ok := runner.LookupTool("gitleaks")
	if !ok {
		t.Fatal("gitleaks is absent from the toolchain")
	}
	t.Setenv("PATH", t.TempDir())
	_, lookupErr := exec.LookPath(tool.Name)
	if lookupErr == nil {
		t.Fatal("missing tool unexpectedly resolved")
	}
	status := collectSetupTool(t.Context(), tool, os.Environ(), nil)
	if status.Path != "" || status.Error != lookupErr.Error() {
		t.Fatalf("missing tool evidence = %+v; want error %q", status, lookupErr)
	}
	line := setupToolLine(status)
	if line.State != "missing" {
		t.Fatalf("missing tool diagnosis = %+v", line)
	}
	requireSetupToolRemedy(t, line, tool)
}

func TestSetupToolAdverseLinesNamePinAndOneRemedy(t *testing.T) {
	t.Parallel()
	tool, ok := runner.LookupTool("gitleaks")
	if !ok {
		t.Fatal("gitleaks is absent from the toolchain")
	}
	for _, tc := range []struct {
		name, path, version, reason, state string
	}{
		{"missing", "", "", "executable file not found in $PATH", "missing"},
		{"other version", "bin/gitleaks", "8.30.0", "", "version"},
		{"failed probe", "bin/gitleaks", "", "probe exited 2", "unknown"},
		{"literal absence with path", "bin/gitleaks", "", "not on PATH", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			line := setupToolLine(ToolStatus{
				Name: tool.Name, Path: tc.path, Version: tc.version, Error: tc.reason,
			})
			if line.State != tc.state {
				t.Fatalf("tool diagnosis = %+v", line)
			}
			requireSetupToolRemedy(t, line, tool)
		})
	}
}

func requireSetupToolRemedy(t *testing.T, line Line, tool runner.Tool) {
	t.Helper()
	if !strings.Contains(line.Detail, tool.Name) || !strings.Contains(line.Detail, tool.Version) ||
		strings.Count(line.Detail, "golaw setup") != 1 {
		t.Fatalf("diagnosis must name the tool, pin, and one setup remedy: %+v", line)
	}
}

func TestSetupToolJSONObservationsReplacePreviousFields(t *testing.T) {
	t.Parallel()
	facts := setupBaseline()
	if err := json.Unmarshal([]byte(`{"tools":[{"name":"lefthook","error":"not on PATH"}]}`), &facts); err != nil {
		t.Fatal(err)
	}
	if len(facts.Tools) != 1 || facts.Tools[0].Path != "" || facts.Tools[0].Version != "" {
		t.Fatalf("new observation retained previous evidence: %+v", facts.Tools)
	}
	if line := setupToolLine(facts.Tools[0]); line.State != "missing" {
		t.Fatalf("missing observation diagnosis = %+v", line)
	}
}

func TestSetupToolJSONRejectsWrongFieldTypesWithoutReplacingEvidence(t *testing.T) {
	t.Parallel()
	status := ToolStatus{Name: "lefthook", Path: "bin/lefthook", Version: "2.1.12"}
	before := status
	err := json.Unmarshal([]byte(`{"path":true}`), &status)
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) || status != before {
		t.Fatalf("invalid observation = %+v, error = %v", status, err)
	}
}
