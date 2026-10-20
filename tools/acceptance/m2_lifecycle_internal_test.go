package main

import (
	"slices"
	"testing"
)

func TestM2Lifecycle(t *testing.T) {
	t.Parallel()
	names := []string{
		"init-unarmed",
		"init-idempotent",
		"dispatch-unarmed",
		"setup-clean",
		"setup-idempotent",
		"setup-interrupted",
		"setup-uninstall",
		"update-preserves-edit",
		"upgrade-commit",
		"pre-commit-disabled",
		"commit-msg-disabled",
		"pre-push-disabled",
		"pre-commit-older",
		"pre-commit-equal",
		"pre-commit-newer",
		"commit-msg-older",
		"commit-msg-equal",
		"commit-msg-newer",
		"pre-push-older",
		"pre-push-equal",
		"pre-push-newer",
	}
	for _, name := range names {
		for _, mode := range []struct {
			name string
			fail bool
		}{
			{"answer", false},
			{"corrupt", true},
			{"broken", true},
		} {
			s := m2TestSuite(t).m2Scope(name + "-" + mode.name)
			script := m2LifecycleScript(t, s, name, mode.name)
			s.execute = script.Execute
			err := s.lifecycleM2(t.Context(), name)
			if script.Calls == 0 {
				t.Fatalf("%s: row did not use its executor", name)
			}
			if mode.name != "broken" && script.Targets == 0 {
				t.Fatalf("%s: lifecycle assertion was not reached", name)
			}
			if (err != nil) != mode.fail {
				t.Fatalf("%s %s: error=%v", name, mode.name, err)
			}
		}
	}
	if !slices.Equal(m2LifecycleNames(), names) {
		t.Fatalf("lifecycle catalog = %q", m2LifecycleNames())
	}
}
