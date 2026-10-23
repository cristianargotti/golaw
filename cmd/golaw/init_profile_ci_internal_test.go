package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
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

// ciBundle installs the fixture bundle without its templates section, a
// profile that leaves no destination to the platform, with the stamp
// carrying the digest of that content.
func ciBundle(t *testing.T, configDir string) {
	t.Helper()
	stamp := profiletest.Install(t, configDir)
	dir := profile.Dir(configDir, profiletest.Name)
	rules := filepath.Join(dir, profile.RulesFile)
	data, err := os.ReadFile(rules)
	if err != nil {
		t.Fatal(err)
	}
	kept := strings.Replace(string(data), "templates: { exclude: [\".github/\"] }\n", "", 1)
	if kept == string(data) {
		t.Fatalf("the fixture rules carry no templates line:\n%s", data)
	}
	if err = os.WriteFile(rules, []byte(kept), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := profile.Digest(dir)
	if err != nil {
		t.Fatal(err)
	}
	stampPath := policy.StampPath(configDir, profiletest.Name)
	text, err := os.ReadFile(stampPath)
	if err == nil {
		err = os.WriteFile(stampPath, []byte(strings.Replace(string(text), stamp.Digest, digest, 1)), 0o600)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// The line reaches the person through the plan init prints: a profile
// that keeps the CI lane names the install, and the fixture, which leaves
// .github/ to the platform, names nothing.
func TestInitPrintsTheCILineOfAProfileThatKeepsTheLane(t *testing.T) {
	t.Parallel()
	ref := profiletest.Ref(t)
	line := "O CI de " + ciWorkflow + " roda make ci, cujo golaw scan precisa do perfil " + ref.Name +
		": execute golaw profile install " + ref.Source + "@" + ref.Version + " no CI"
	for name, keeps := range map[string]bool{"red a profile that keeps the lane": true, "near the lane left to the platform": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, _ := initApp(t)
			dirs, err := ta.bind().dirs()
			if err != nil {
				t.Fatal(err)
			}
			if keeps {
				ciBundle(t, dirs.Config)
			} else {
				profiletest.Install(t, dirs.Config)
			}
			root := checkout(t)
			if code := ta.run(t.Context(), []string{"init", "--profile", profiletest.Name, root}); code != exitClean {
				t.Fatalf("exit = %d: %s", code, ta.stderr.String())
			}
			if got := strings.Contains(ta.stdout.String(), line); got != keeps {
				t.Errorf("the CI line printed = %v, want %v:\n%s", got, keeps, ta.stdout.String())
			}
		})
	}
}
