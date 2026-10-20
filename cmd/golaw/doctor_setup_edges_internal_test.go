package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestSetupLinesCopySpareCapacityAndReportHeaders(t *testing.T) {
	t.Parallel()
	backing := []doctor.Line{{ID: "agent"}, {ID: "spare"}, {ID: "tail"}}
	reports := []doctor.Report{{Schema: 1, Lines: backing[:1]}}
	lines := []doctor.Line{{ID: "setup"}}
	got := withSetupLines(reports, lines)
	if len(got[0].Lines) != 2 || got[0].Lines[1].ID != "setup" || backing[1].ID != "spare" {
		t.Fatalf("merge reused input capacity: got = %+v, input = %+v", got, backing)
	}
	got[0].Schema = 2
	got[0].Lines[0].ID = "changed agent"
	got[0].Lines[1].ID = "changed setup"
	if reports[0].Schema != 1 || reports[0].Lines[0].ID != "agent" || lines[0].ID != "setup" {
		t.Fatalf("merged reports alias their inputs: reports = %+v, lines = %+v", reports, lines)
	}
	if empty := withSetupLines(nil, lines); empty != nil {
		t.Fatalf("no agent report became %+v", empty)
	}
	if empty := withSetupLines([]doctor.Report{}, lines); empty == nil || len(empty) != 0 {
		t.Fatalf("empty agent reports became %+v", empty)
	}
}

func TestSetupDoctorKeepsAnUnarmedCheckoutOptional(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := gittest.New(t).Root
	for _, configured := range []bool{false, true} {
		if configured {
			data := strings.Replace(string(renderedPolicy(t)),
				"ladder: { pre_commit: true, commit_msg: true, pre_push: true,",
				"ladder: { pre_commit: false, commit_msg: false, pre_push: false,", 1)
			if err := os.WriteFile(filepath.Join(root, policy.FileName), []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		lines, err := b.setupDoctor(t.Context(), root)
		if err != nil || len(lines) != 1 || lines[0].ID != "doctor.setup-receipt" || lines[0].State != "not-armed" {
			t.Fatalf("unarmed checkout, policy present %t: %+v, %v", configured, lines, err)
		}
	}
}

// Setup refuses a directory outside git, so doctor names no setup line
// there, and no policy is in force to name a managed file.
func TestDoctorLeavesRepositoryLinesOutsideGit(t *testing.T) {
	t.Parallel()
	for _, asJSON := range []bool{false, true} {
		t.Run(map[bool]string{false: "text", true: "json"}[asJSON], func(t *testing.T) {
			t.Parallel()
			ta, _ := doctorApp(t)
			ta.discover = ta.bind().discover
			root := t.TempDir()
			writeFiles(t, root, map[string]string{
				policy.FileName: managedDoctorPolicy(t, "recorded\n"),
				"golaw.mk":      "edited\n",
			})
			args := []string{"doctor", root}
			if asJSON {
				args = append(args, "--json")
			}
			if code := ta.run(t.Context(), args); code != exitClean || ta.stderr.Len() != 0 {
				t.Fatalf("outside git: exit = %d, stderr = %q", code, ta.stderr.String())
			}
			out := ta.stdout.String()
			if !strings.Contains(out, "doctor.plugin") || strings.Contains(out, "doctor.setup-") ||
				strings.Contains(out, "doctor.managed") {
				t.Fatalf("outside git the report names repository lines: %q", out)
			}
		})
	}
}

func TestDoctorSetupCollectionFailurePrintsNoPartialReport(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"state", "root"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			ta, _ := doctorApp(t)
			root := t.TempDir()
			b := ta.bind()
			reason := "resolve the state directories"
			if failure == "state" {
				b.env = func(string) string { return "" }
			} else {
				root = filepath.Join(root, "absent")
				reason = "absent"
			}
			ta.bind = func() binding { return b }
			code := ta.run(t.Context(), []string{"doctor", root})
			if code != exitFailure || !strings.Contains(ta.stderr.String(), reason) || ta.stdout.Len() != 0 {
				t.Fatalf("collection failure = %d, stdout = %q, stderr = %q", code, ta.stdout, ta.stderr)
			}
		})
	}
}

type doctorFailWriter struct{ remaining int }

func (w *doctorFailWriter) Write(data []byte) (int, error) {
	if w.remaining == 0 {
		return 0, errBoom
	}
	w.remaining--
	return len(data), nil
}

func TestDoctorPreservesOutputFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		remaining int
		asJSON    bool
	}{
		{"header", 0, false},
		{"text", 1, false},
		{"json", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ta, _ := doctorApp(t)
			err := ta.doctor(t.Context(), &doctorFailWriter{remaining: tc.remaining}, t.TempDir(), scopeUser, tc.asJSON)
			if !errors.Is(err, errBoom) {
				t.Fatalf("output failure lost its cause: %v", err)
			}
		})
	}
}
