package main

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestDoctorInputNamesTheAgentAndItsDirectories(t *testing.T) {
	t.Parallel()
	b, home := testBinding(t)
	root := repository(t, true)
	agent := agentUnderTest()
	input, err := b.doctorInput(t.Context(), agent, root, scopeProject, now())
	if err != nil {
		t.Fatal(err)
	}
	state, err := b.installState()
	if err != nil {
		t.Fatal(err)
	}
	if input.Agent != agent.ID || input.Scope != scopeProject || input.Home != home || input.StateDir != state {
		t.Fatalf("input = %+v", input)
	}
	if input.ConfigDir != agent.ConfigDir(b.env, home) {
		t.Fatalf("directories = %+v", input)
	}
}

func TestDoctorInputCarriesTheLanguageOfThePolicy(t *testing.T) {
	t.Parallel()
	b, _ := testBinding(t)
	input, err := b.doctorInput(t.Context(), agentUnderTest(), repository(t, true), scopeUser, now())
	if err != nil || input.Language != model.DefaultLanguage {
		t.Fatalf("language = %q, %v", input.Language, err)
	}
	if got := b.doctorLanguage(t.Context(), t.TempDir(), now()); got != model.DefaultLanguage {
		t.Fatalf("a directory without a policy = %q", got)
	}
}

// TestDoctorExitsCleanOnceItPrintsRedLines pins the exit codes of M1:
// doctor answers 0 once every report is printed, each ending with its
// sentence for the person, and an agent the person never installed is
// reported unknown, never red, so an adopter of one agent reads no finding
// about the others.
func TestDoctorExitsCleanOnceItPrintsRedLines(t *testing.T) {
	t.Parallel()
	ta := bindApp(t)
	ta.doctorReport = ta.bind().doctorReport
	ta.discover = ta.bind().discover
	root := repository(t, true)
	if got := ta.run(t.Context(), []string{"doctor", root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, ta.stderr.String())
	}
	out := ta.stdout.String()
	for _, id := range agents.IDs() {
		if !strings.Contains(out, "doctor.state "+id+"/user: unknown [not-installed]") ||
			!strings.Contains(out, "golaw: "+id+" não está instalado neste escopo; para usá-lo, rode golaw agent install --agent "+id+".") {
			t.Errorf("the report of %s is missing from %q", id, out)
		}
	}
}
