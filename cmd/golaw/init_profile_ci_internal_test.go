package main

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// A profile that keeps the managed CI lane names the install the lane
// runs before make ci; one that leaves .github/ to the platform, and core
// only, name nothing.
func TestProfileCILineNamesTheInstallTheCIRuns(t *testing.T) {
	t.Parallel()
	ref := profiletest.Ref(t)
	pinned := []byte("golaw: 1\nkit: { version: 0.1.0 }\n" + pinLine(ref) + "\nlanguage: pt-BR\n")
	lane := bootstrap.TemplatePlan{Changes: []install.Change{{Path: ciWorkflow}}}
	managed := bootstrap.TemplatePlan{Managed: []policy.ManagedFile{{Path: ciWorkflow}}}
	portuguese := &model.EffectivePolicy{Language: model.DefaultLanguage}
	english := &model.EffectivePolicy{Language: "en"}
	command := "golaw profile install " + ref.Source + "@" + ref.Version
	cases := map[string]struct {
		plan initPlan
		want string
	}{
		"red a pin and a new lane": {
			initPlan{data: pinned, templates: lane, policy: portuguese},
			"O CI de .github/workflows/ci.yml roda make ci, cujo golaw scan precisa do perfil fixture: execute " + command + " no CI, com uma credencial que leia o bundle, antes de make ci.",
		},
		"red a pin and a managed lane": {
			initPlan{data: pinned, templates: managed, policy: english},
			"The CI of .github/workflows/ci.yml runs make ci, whose golaw scan needs profile fixture: run " + command + " in the CI, with a credential that reads the bundle, before make ci.",
		},
		"near the lane left to the platform": {initPlan{data: pinned, templates: bootstrap.TemplatePlan{}, policy: portuguese}, ""},
		"near core only":                     {initPlan{data: []byte("golaw: 1\n"), templates: lane, policy: portuguese}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := profileCILine(tc.plan); got != tc.want {
				t.Fatalf("profileCILine = %q, want %q", got, tc.want)
			}
		})
	}
}
