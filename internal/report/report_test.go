package report_test

import (
	"context"
	"testing"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
	"github.com/cristianargotti/golaw/internal/report"
)

// fake is a check whose result is fixed, so a run carries every status.
type fake struct {
	id       string
	owner    string
	requires []string
	result   model.Result
}

func (f fake) Spec() model.Spec {
	owner := f.owner
	if owner == "" {
		owner = model.OwnerCore
	}
	return model.Spec{ID: f.id, Rule: "rules/4.1@1", Scope: model.ScopeFile, Owner: owner, Proof: f.id, Requires: f.requires}
}

func (f fake) Run(context.Context, model.Input) (model.Result, error) { return f.result, nil }

// run dispatches four fakes in scan mode: a denial with a place, a note
// without a path, an unmeasured check and a check that cannot run in scan.
func run(t *testing.T) (*engine.Run, engine.Decision) {
	t.Helper()
	b := engine.New()
	checks := []model.Check{
		fake{id: "text.dashes", result: model.Result{Status: model.Measured(), Scope: "2 files", Findings: []model.Finding{
			{Path: "docs/a.md", Line: 3, Column: 7, Code: "em-dash", Message: "em dash in text", Severity: model.SeverityDeny},
		}}},
		fake{id: "shell.push", result: model.Result{Status: model.Measured(), Scope: "1 command", Findings: []model.Finding{
			{Code: "push", Message: "git push observed", Severity: model.SeverityNote},
		}}},
		fake{id: "text.todo", result: model.Result{Status: model.Unmeasured("read a.go: path unreadable")}},
		fake{id: "goast.goleak", requires: []string{"external-evidence"}},
		fake{id: "goast.sample", owner: "sample"},
	}
	for _, c := range checks {
		if err := b.Register(c); err != nil {
			t.Fatal(err)
		}
	}
	reg, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	tree := proof.NewTree(map[string][]byte{"docs/a.md": []byte("x\n")})
	r, err := engine.Dispatch(t.Context(), reg, model.Input{Mode: model.ModeScan, Policy: proof.Policy(), Snapshot: tree, Targets: tree.Files()})
	if err != nil {
		t.Fatal(err)
	}
	return r, engine.Decide(r)
}

func identity() report.Identity {
	return report.Identity{KitVersion: "0.1.0", PolicyDigest: "policy-digest", Ruleset: "ruleset-digest", Snapshot: "worktree:abc123"}
}

func TestNewCountsEverything(t *testing.T) {
	t.Parallel()
	r, d := run(t)
	doc := report.New(r, d, identity())
	if doc.Schema != 1 || doc.Mode != "scan" || doc.Verdict != "fail" || doc.Snapshot != "worktree:abc123" || doc.Policy != "policy-digest" {
		t.Errorf("document = %+v", doc)
	}
	if doc.Counts != (report.Counts{Findings: 2, Enforced: 1, Notes: 1, Excluded: 1, Unmeasured: 2}) {
		t.Errorf("counts = %+v", doc.Counts)
	}
	if len(doc.Checks) != 5 || doc.Checks[0].ID != "goast.goleak" || doc.Checks[0].Status != "unmeasured" || doc.Checks[3].Scope != "2 files" {
		t.Errorf("checks = %+v", doc.Checks)
	}
	if doc.Checks[1].Status != "not-applicable" || doc.Checks[1].Reason != "profile sample is not enrolled" {
		t.Errorf("excluded check = %+v", doc.Checks[1])
	}
	// The engine sorts by path first, so the event finding without a path
	// comes before the file finding.
	if doc.Findings[0].Fingerprint == "" || doc.Findings[0].Code != "push" || doc.Findings[1].Check != "text.dashes" {
		t.Errorf("findings = %+v", doc.Findings)
	}
	empty := report.New(nil, engine.Decision{Mode: model.ModeScan, Verdict: engine.VerdictClean}, identity())
	if empty.Findings == nil || len(empty.Checks) != 0 || empty.Counts != (report.Counts{}) {
		t.Errorf("empty document = %+v", empty)
	}
}

func TestRulesetDigestOrdersByID(t *testing.T) {
	t.Parallel()
	a, b := fake{id: "text.a"}, fake{id: "text.b"}
	if report.RulesetDigest([]model.Check{a, b}) != report.RulesetDigest([]model.Check{b, a}) {
		t.Error("the digest depends on the registration order")
	}
	if report.RulesetDigest([]model.Check{a}) == report.RulesetDigest([]model.Check{b}) || len(report.RulesetDigest(nil)) != 64 {
		t.Error("the digest ignores the ids")
	}
}

func TestIntroducedComparesMultisets(t *testing.T) {
	t.Parallel()
	one := model.Finding{Check: "text.dashes", Path: "a.md", Code: "em-dash", Message: "em dash", Line: 3}
	two := one
	two.Line = 9
	one.Fingerprint, two.Fingerprint = model.Fingerprint(one), model.Fingerprint(two)
	if one.Fingerprint != two.Fingerprint {
		t.Fatal("a line shift changed the fingerprint")
	}
	if got := report.Introduced([]model.Finding{one, two}, []model.Finding{one}); len(got) != 1 || got[0].Line != 9 {
		t.Errorf("a second occurrence is new: %+v", got)
	}
	if got := report.Introduced([]model.Finding{two}, []model.Finding{one}); len(got) != 0 {
		t.Errorf("a shifted occurrence is not new: %+v", got)
	}
	if m := report.Multiset([]model.Finding{one, two}); m[one.Fingerprint] != 2 {
		t.Errorf("multiset = %v", m)
	}
}
