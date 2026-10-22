package main

import "testing"

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
