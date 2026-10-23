package policyfiles_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// TestBuildChecksLeaveAnUnrelatedDoubleColonRuleSilent holds the reading of
// a double-colon rule to D2: the near fixture of every build check keeps its
// answer beside an unrelated clean:: rule, while a check that needs a target
// only a double-colon rule defines states a limit naming that rule.
func TestBuildChecksLeaveAnUnrelatedDoubleColonRuleSilent(t *testing.T) {
	t.Parallel()
	for id, check := range map[string]model.Check{
		"policy.makefile": policyfiles.Makefile(), "policy.ci-target": policyfiles.CITarget(),
		"policy.generated-fresh": policyfiles.GeneratedFresh(), "policy.gotoolchain-local": policyfiles.GotoolchainLocal(),
		"policy.deps-age-config": policyfiles.DepsAgeConfig(),
	} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			loaded, err := proof.LoadFixture(filepath.Join("..", "..", "proof", "testdata", id, "near"))
			if err != nil {
				t.Fatal(err)
			}
			files := make(map[string]string, len(loaded.Files))
			for name, content := range loaded.Files {
				files[name] = string(content)
			}
			want := measure(t, check, fixture{files: files, capabilities: []string{"go"}})
			files["Makefile"] += "\nclean::\n\tgo clean -cache\n"
			got := measure(t, check, fixture{files: files, capabilities: []string{"go"}})
			if got.Status.Kind != want.Status.Kind || len(got.Findings) != len(want.Findings) {
				t.Fatalf("beside clean:: = %+v, without it %+v", got, want)
			}
		})
	}
}

func TestCITargetNamesTheDoubleColonRuleItNeeds(t *testing.T) {
	t.Parallel()
	got := measure(t, policyfiles.CITarget(), fixture{files: map[string]string{"Makefile": "lint:\n\nci:: lint\n"}, capabilities: []string{"go"}})
	if !model.IsOutsideSubset(got.Status) || !strings.Contains(got.Status.Reason, "Makefile:3: double-colon rule ci::") {
		t.Fatalf("ci defined by a double-colon rule = %+v", got)
	}
}
