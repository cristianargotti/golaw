package proof_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// neededCheck is a dash check that applies only where its capabilities hold.
type neededCheck struct {
	dashCheck

	needs []string
}

func (n neededCheck) Spec() model.Spec {
	s := n.dashCheck.Spec()
	s.Needs = n.needs
	return s
}

func TestCapabilityHoldsForACheckThatAppliesOnlyWhereItsCapabilitiesHold(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	m := manifest(t, root)
	reg := registry(t, neededCheck{dashCheck{id: "text.dashes"}, []string{"go", "api"}}, noVerifyCheck{})
	if failures := proof.Capability(t.Context(), reg, root, m); len(failures) != 0 {
		t.Fatalf("Capability() = \n%s", reasons(failures))
	}
	if failures := proof.Verify(t.Context(), reg, root, m); len(failures) != 0 {
		t.Fatalf("Verify() = \n%s", reasons(failures))
	}
}

func TestCapabilityFailsWhenTheRedCaseIsJudgedWithoutTheCapability(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	m := manifest(t, root)
	cases := map[string]struct {
		twin model.Check
		want []string
	}{
		"a twin with no need":        {dashCheck{id: "text.twin"}, []string{"with capability go off text.twin still reports em-dash", "with capability api off text.twin still reports em-dash"}},
		"a twin that needs api only": {neededCheck{dashCheck{id: "text.twin"}, []string{"api"}}, []string{"with capability go off text.twin still reports em-dash"}},
		"a twin with the same needs": {neededCheck{dashCheck{id: "text.twin"}, []string{"api", "go"}}, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			reg := registry(t, neededCheck{dashCheck{id: "text.dashes"}, []string{"go", "api"}}, tc.twin, noVerifyCheck{})
			failures := proof.Capability(t.Context(), reg, root, m)
			if len(failures) != len(tc.want) {
				t.Fatalf("Capability() = \n%s", reasons(failures))
			}
			for i, f := range failures {
				if f.Pass != proof.PassCapability || f.ID != "text.dashes" || !strings.Contains(f.Reason, tc.want[i]) {
					t.Errorf("failure %d = %v, want %q", i, f, tc.want[i])
				}
			}
		})
	}
}

func TestCapabilityLeavesACheckWithoutNeedsToTheOtherPasses(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	twin := registry(t, dashCheck{id: "text.dashes"}, dashCheck{id: "text.twin"}, noVerifyCheck{})
	if failures := proof.Capability(t.Context(), twin, root, manifest(t, root)); len(failures) != 0 {
		t.Fatalf("Capability() = \n%s", reasons(failures))
	}
}

func TestCapabilityFailsOnAnEntryItCannotLoad(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	reg := registry(t, neededCheck{dashCheck{id: "text.dashes"}, []string{"go"}}, noVerifyCheck{})
	cases := map[string]struct {
		entry proof.Entry
		want  string
	}{
		"unregistered":   {proof.Entry{ID: "text.todo", Code: "todo", Red: "x", Near: "y"}, "not registered"},
		"absent fixture": {proof.Entry{ID: "text.dashes", Code: "em-dash", Red: "testdata/absent", Near: "testdata/text.dashes/near"}, "invalid proof fixture"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			failures := proof.Capability(t.Context(), reg, root, proof.Manifest{Checks: []proof.Entry{tc.entry}})
			if len(failures) != 1 || failures[0].Pass != proof.PassCapability || !strings.Contains(failures[0].Reason, tc.want) {
				t.Fatalf("Capability() = %v", failures)
			}
		})
	}
}
