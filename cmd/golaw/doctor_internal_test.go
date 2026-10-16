package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook/agents"
)

// doctorApp wires the doctor command with a fake report and records the
// root and scope it was asked about.
func doctorApp(t *testing.T) (testApp, *[]string) {
	t.Helper()
	ta := newTestApp(t)
	asked := &[]string{}
	ta.doctorReport = func(_ context.Context, _ agents.Agent, root, scope string, _ time.Time) (doctor.Report, error) {
		*asked = append(*asked, root, scope)
		return doctor.Report{Schema: 1, Lines: []doctor.Line{{ID: "doctor.plugin", State: "ok", Code: "plugin-enabled", Detail: "configured"}}}, nil
	}
	return ta, asked
}

func TestDoctorPrintsOneReportPerAgent(t *testing.T) {
	t.Parallel()
	ta, asked := doctorApp(t)
	root := t.TempDir()
	if got := ta.run(t.Context(), []string{"doctor", root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	out := ta.stdout.String()
	if !strings.HasPrefix(out, "agent: "+agents.IDs()[0]+"\n") || !strings.Contains(out, "doctor.plugin") || (*asked)[0] != root || (*asked)[1] != scopeUser {
		t.Fatalf("stdout = %q, asked = %v", out, *asked)
	}
	ta.stdout.Reset()
	if got := ta.run(t.Context(), []string{"doctor", "--json", "--scope", scopeProject, root}); got != exitClean {
		t.Fatalf("json exit = %d, stderr = %q", got, ta.stderr.String())
	}
	var report doctor.Report
	if err := json.Unmarshal(ta.stdout.Bytes(), &report); err != nil || report.Schema != 1 || len(report.Lines) != 1 || (*asked)[3] != scopeProject {
		t.Fatalf("json = %q, %v", ta.stdout.String(), err)
	}
}

func TestDoctorStandsOnTheRealPathOutsideGit(t *testing.T) {
	t.Parallel()
	ta, asked := doctorApp(t)
	ta.discover = func(context.Context, string) (string, error) { return "", errBoom }
	if got := ta.run(t.Context(), []string{"doctor", "."}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	want, err := filepath.Abs(".")
	if err != nil || (*asked)[0] != want {
		t.Fatalf("root = %q, want %q", (*asked)[0], want)
	}
}

func TestDoctorReportsFailures(t *testing.T) {
	t.Parallel()
	ta, _ := doctorApp(t)
	ta.doctorReport = func(context.Context, agents.Agent, string, string, time.Time) (doctor.Report, error) {
		return doctor.Report{}, errBoom
	}
	if got := ta.run(t.Context(), []string{"doctor"}); got != exitFailure || ta.stderr.String() != "golaw: doctor: boom\n" {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	scope, _ := doctorApp(t)
	if got := scope.run(t.Context(), []string{"doctor", "--scope", "team"}); got != exitFailure || !strings.Contains(scope.stderr.String(), "--scope must be") {
		t.Fatalf("exit = %d, stderr = %q", got, scope.stderr.String())
	}
}
