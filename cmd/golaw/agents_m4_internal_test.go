package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
)

// The agents of the registry by position, so this file names none: the
// first answers through exit codes and stderr, the second through exit 2
// and stderr, the third through the JSON it always prints.
func registryIDs(t *testing.T) (string, string, string) {
	t.Helper()
	ids := agents.IDs()
	if len(ids) != 3 {
		t.Fatalf("ids = %v", ids)
	}
	return ids[0], ids[1], ids[2]
}

func TestHookOfTheAgentsNotBuiltFailsClosed(t *testing.T) {
	t.Parallel()
	_, second, third := registryIDs(t)
	for id, want := range map[string]int{second: exitFailure, third: exitClean} {
		ta := newTestApp(t)
		ta.stdin = bytes.NewReader([]byte(`{"hook_event_name":"PreToolUse"}`))
		ta.hookDeps = func(agents.Agent, time.Time) (hook.Dependencies, error) { return hook.Dependencies{}, nil }
		got := ta.run(t.Context(), []string{"hook", id})
		denied := strings.Contains(ta.stdout.String(), `"permission":"deny"`) || strings.Contains(ta.stderr.String(), "cannot judge")
		if got != want || !denied {
			t.Errorf("%s: exit %d stdout %q stderr %q", id, got, ta.stdout.String(), ta.stderr.String())
		}
	}
}

func TestAgentInstallAllStopsAtTheFirstAgentNotBuilt(t *testing.T) {
	t.Parallel()
	first, second, third := registryIDs(t)
	ta, port, root := agentApp(t)
	if got := ta.run(t.Context(), []string{"agent", "install", "--agent", "all", root}); got != exitFailure {
		t.Fatalf("exit = %d", got)
	}
	out := ta.stdout.String()
	if !strings.Contains(out, "agent: "+first+"\n") || !strings.Contains(out, "agent: "+second+"\n") || strings.Contains(out, "agent: "+third+"\n") || port.calls != 1 {
		t.Fatalf("stdout = %q, port = %+v", out, port)
	}
	if !strings.Contains(ta.stderr.String(), install.ErrNotBuilt.Error()) {
		t.Fatalf("stderr = %q", ta.stderr.String())
	}
}

func TestAgentUninstallAllSkipsTheAgentsWithoutAReceipt(t *testing.T) {
	t.Parallel()
	first, second, third := registryIDs(t)
	ta, _, root := agentApp(t)
	if got := ta.run(t.Context(), []string{"agent", "install", "--agent", first, root}); got != exitClean {
		t.Fatalf("install exit = %d, stderr = %q", got, ta.stderr.String())
	}
	ta.stdout.Reset()
	if got := ta.run(t.Context(), []string{"agent", "uninstall", "--agent", "all", root}); got != exitClean {
		t.Fatalf("uninstall exit = %d, stderr = %q", got, ta.stderr.String())
	}
	out := ta.stdout.String()
	if !strings.Contains(out, "uninstalled: ") || !strings.Contains(out, "not installed: "+second) || !strings.Contains(out, "not installed: "+third) {
		t.Fatalf("stdout = %q", out)
	}
}

func TestDoctorReportsComeFromTheAgentOfEachReport(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	root := repository(t, true)
	for _, agent := range agents.All()[1:] {
		report, err := b.doctorReport(t.Context(), agent, root, scopeUser, now())
		if err != nil || len(report.Lines) != 1 || report.Lines[0].ID != "doctor.agent" || report.Lines[0].Key != agent.ID {
			t.Fatalf("%s: %+v, %v", agent.ID, report, err)
		}
	}
}

func TestInstallEndsWithTheNotesOfItsPlan(t *testing.T) {
	t.Parallel()
	plan := install.Plan{ReceiptPath: "receipt.json", Notes: []string{"trust the hooks: run /hooks"}}
	got := installedLines(plan)
	if len(got) != 2 || got[0] != "installed: receipt.json" || got[1] != plan.Notes[0] {
		t.Fatalf("lines = %q", got)
	}
}
