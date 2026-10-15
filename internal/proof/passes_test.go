package proof_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestVerifyHoldsForProvenChecks(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	reg := registry(t, dashCheck{id: "text.dashes"}, noVerifyCheck{})
	if failures := proof.Verify(t.Context(), reg, root, manifest(t, root)); len(failures) != 0 {
		t.Fatalf("Verify() = \n%s", reasons(failures))
	}
	p := proof.Policy()
	if len(p.Capabilities) != len(model.Capabilities()) || p.Enrollment.State != model.EnrollmentEnrolled || p.Digest == "" {
		t.Errorf("Policy() = %+v", p)
	}
}

func TestCompletenessBothWays(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	m := manifest(t, root)
	extra := proof.Completeness(registry(t, dashCheck{id: "text.dashes"}, noVerifyCheck{}, dashCheck{id: "text.todo"}), m)
	if len(extra) != 1 || extra[0].ID != "text.todo" || extra[0].Pass != proof.PassCompleteness || !strings.Contains(extra[0].Reason, "no manifest entry") {
		t.Errorf("extra registration = %v", extra)
	}
	missing := proof.Completeness(registry(t, dashCheck{id: "text.dashes"}), m)
	if len(missing) != 1 || missing[0].ID != "shell.no-verify" || !strings.Contains(missing[0].Reason, "no registered check") {
		t.Errorf("missing registration = %v", missing)
	}
}

func TestBaselineFailures(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	reg := registry(t, dashCheck{id: "text.dashes"}, noVerifyCheck{})
	swapped := proof.Manifest{Checks: []proof.Entry{{ID: "text.dashes", Code: "em-dash", Red: "testdata/text.dashes/near", Near: "testdata/text.dashes/red"}}}
	failures := proof.Baseline(t.Context(), reg, root, swapped)
	if got := reasons(failures); len(failures) != 2 || !strings.Contains(got, "yields no finding with code em-dash") || !strings.Contains(got, "near miss yields em-dash") {
		t.Errorf("swapped fixtures: %s", got)
	}
	unregistered := proof.Manifest{Checks: []proof.Entry{{ID: "text.todo", Code: "todo", Red: "x", Near: "y"}}}
	if failures := proof.Baseline(t.Context(), reg, root, unregistered); len(failures) != 1 || !strings.Contains(failures[0].Reason, "not registered") {
		t.Errorf("unregistered: %v", failures)
	}
	absent := proof.Manifest{Checks: []proof.Entry{{ID: "text.dashes", Code: "em-dash", Red: "testdata/absent", Near: "testdata/text.dashes/near"}}}
	if failures := proof.Baseline(t.Context(), reg, root, absent); len(failures) != 1 || !strings.Contains(failures[0].Reason, "invalid proof fixture") {
		t.Errorf("absent fixture: %v", failures)
	}
	unmeasured := registry(t, dashCheck{id: "text.dashes", requires: []string{"external-evidence"}}, noVerifyCheck{})
	if failures := proof.Baseline(t.Context(), unmeasured, root, manifest(t, root)); len(failures) != 2 || !strings.Contains(failures[0].Reason, "the check was unmeasured") {
		t.Errorf("unmeasured check: %v", failures)
	}
}

func TestDeletionFailsWhenAnotherCheckReportsTheCode(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	m := manifest(t, root)
	twin := registry(t, dashCheck{id: "text.dashes"}, dashCheck{id: "text.twin"}, noVerifyCheck{})
	failures := proof.Deletion(t.Context(), twin, root, m)
	if len(failures) != 1 || failures[0].ID != "text.dashes" || failures[0].Pass != proof.PassDeletion || !strings.Contains(failures[0].Reason, "text.twin still reports em-dash") {
		t.Errorf("twin: %v", failures)
	}
	if failures := proof.Deletion(t.Context(), registry(t, dashCheck{id: "text.dashes"}, noVerifyCheck{}), root, m); len(failures) != 0 {
		t.Errorf("independent checks: %v", failures)
	}
	unregistered := proof.Manifest{Checks: []proof.Entry{{ID: "text.todo", Code: "todo", Red: "x", Near: "y"}}}
	if failures := proof.Deletion(t.Context(), twin, root, unregistered); len(failures) != 1 || failures[0].Pass != proof.PassDeletion {
		t.Errorf("unregistered: %v", failures)
	}
}

func TestOverFireFailsWhenTheNearMissNeverReachesTheCheck(t *testing.T) {
	t.Parallel()
	root := fixtureRoot(t)
	m := manifest(t, root)
	unreached := registry(t, dashCheck{id: "text.dashes", requires: []string{"external-evidence"}}, noVerifyCheck{})
	failures := proof.OverFire(t.Context(), unreached, root, m)
	if len(failures) != 1 || failures[0].ID != "text.dashes" || failures[0].Pass != proof.PassOverFire || !strings.Contains(failures[0].Reason, "never reaches the check (unmeasured") {
		t.Errorf("unreached: %v", failures)
	}
	unregistered := proof.Manifest{Checks: []proof.Entry{{ID: "text.todo", Code: "todo", Red: "x", Near: "y"}}}
	if failures := proof.OverFire(t.Context(), unreached, root, unregistered); len(failures) != 1 || failures[0].Pass != proof.PassOverFire {
		t.Errorf("unregistered: %v", failures)
	}
	if f := (proof.Failure{Pass: "baseline", ID: "x.y", Reason: "why"}); f.String() != "baseline x.y: why" {
		t.Errorf("Failure.String() = %q", f.String())
	}
}

func reasons(failures []proof.Failure) string {
	out := make([]string, 0, len(failures))
	for _, f := range failures {
		out = append(out, f.String())
	}
	return strings.Join(out, "\n")
}
