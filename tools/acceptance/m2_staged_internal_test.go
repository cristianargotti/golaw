package main

import (
	"slices"
	"testing"
)

func TestM2StagedStates(t *testing.T) {
	t.Parallel()
	names := []string{
		"partial-red", "partial-near",
		"opposite-red", "opposite-near",
		"deletion-red", "deletion-near",
		"rename-red", "rename-near",
		"dirty-red", "dirty-near",
		"alternate-red", "alternate-near",
		"policy-red", "policy-near",
		"length-red", "length-near",
	}
	modes := []struct {
		name string
		fail bool
	}{
		{"answer", false},
		{"corrupt", true},
		{"broken", true},
		{"mutation", true},
	}
	for _, name := range names {
		for _, mode := range modes {
			s := m2TestSuite(t).m2Scope(name + "-" + mode.name)
			script := m2StagedScript(t, s, name, mode.name)
			s.execute = script.Execute
			err := s.stagedM2(t.Context(), name)
			if script.Calls == 0 {
				t.Fatalf("%s: row did not use its executor", name)
			}
			if mode.name != "broken" && script.Targets == 0 {
				t.Fatalf("%s: staged command was not executed", name)
			}
			if (err != nil) != mode.fail {
				t.Fatalf("%s %s: error=%v", name, mode.name, err)
			}
		}
	}
	if !slices.Equal(m2StagedNames(), names) {
		t.Fatalf("staged catalog = %q", m2StagedNames())
	}
}
