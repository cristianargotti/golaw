package rung

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/checks"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runnertest"
)

// statedLimit is the status a policy check answers for a construct outside
// the subset its reader declares: the reader's error, which wraps the
// sentinel after the construct, as the Unmeasured reason.
func statedLimit(what string) model.Status {
	return model.Unmeasured(fmt.Errorf("%s: %w", what, model.ErrOutsideSubset).Error())
}

// limitRun is a staged run whose policy check met a construct outside the
// subset its reader declares, beside a check that measured.
func limitRun(status model.Status) *engine.Run {
	return &engine.Run{Mode: model.ModeStaged, Outcomes: []engine.Outcome{
		{Spec: model.Spec{ID: "policy.makefile", Rule: "rules/4.13@1"}, Result: model.Result{Status: status}},
		{Spec: model.Spec{ID: "text.dashes", Rule: "rules/4.1@1"}, Result: model.Result{Status: model.Measured()}},
	}}
}

// TestRungLeavesAStatedLimitToThePipeline pins contract D2 at the rungs: a
// construct outside the declared subset is no finding the rung recognizes,
// so it is named in one line and the commit goes on, while any other check
// left unmeasured still refuses it.
func TestRungLeavesAStatedLimitToThePipeline(t *testing.T) {
	t.Parallel()
	limit := statedLimit("Makefile:3: $(shell) in the prerequisites of ci")
	out := judged(PreCommit, model.DefaultLanguage, limitRun(limit))
	want := "golaw pre-commit: policy.makefile left to the pipeline: Makefile:3: $(shell) in the prerequisites of ci: outside the declared subset, a stated limit; docs/rules.md names the rewrite"
	if out.Code != codeClean || !slices.Contains(out.Lines, want) {
		t.Fatalf("a stated limit = %+v", out)
	}
	for name, status := range map[string]model.Status{
		"unreadable":    model.Unmeasured("read Makefile: path unreadable"),
		"limit wording": model.Unmeasured("Makefile:3: " + model.ErrOutsideSubset.Error() + " in a recipe"),
	} {
		if got := judged(PreCommit, model.DefaultLanguage, limitRun(status)); got.Code != codeFailure {
			t.Errorf("%s = %+v, want the failure code", name, got)
		}
	}
}

func TestLimitedNamesEachCheckOnce(t *testing.T) {
	t.Parallel()
	run := limitRun(statedLimit("Makefile:3: an include of a computed name"))
	run.Outcomes = append(run.Outcomes, engine.Outcome{
		Spec:   model.Spec{ID: "policy.ci-target", Rule: "rules/4.13@1"},
		Result: model.Result{Status: statedLimit("Makefile:3: an include of a computed name")},
	})
	limits := limited(run)
	if len(limits) != 2 || run.Outcomes[0].Result.Status.Kind != model.StatusMeasured || run.Outcomes[2].Result.Status.Kind != model.StatusMeasured {
		t.Fatalf("limits = %q, outcomes = %+v", limits, run.Outcomes)
	}
	if lines := limitLines(PreCommit, nil); len(lines) != 0 {
		t.Errorf("no limit = %q", lines)
	}
}

// limitCheck is a tree check that answers the status it holds, as a
// policy check does over the index; its id stays outside the policy family
// so no check a later task registers collides with it.
type limitCheck struct{ status model.Status }

func (limitCheck) Spec() model.Spec {
	return model.Spec{ID: "fixture.stated-limit", Rule: "rules/4.13@1", Scope: model.ScopeTree, Owner: model.OwnerCore, Proof: "fixture.stated-limit"}
}

func (c limitCheck) Run(context.Context, model.Input) (model.Result, error) {
	return model.Result{Status: c.status, Scope: "Makefile"}, nil
}

// TestPreCommitLeavesAStatedLimitToThePipeline pins the stated limit on the
// rung that dispatches the tree checks over the index: the staged scan of
// the pre-commit rung counts it measured and names it after the scan, so
// the commit goes on, while any other check left unmeasured still refuses.
// It runs alone because the fake gitleaks is found through PATH.
func TestPreCommitLeavesAStatedLimitToThePipeline(t *testing.T) {
	dir := t.TempDir()
	runnertest.Install(t, dir, "gitleaks", runnertest.Behavior{Probe: []string{"version"}, Version: "8.30.1"})
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	r := repository(t, pinned("0.1.0", ""))
	r.Stage(map[string]string{"notes.md": "why\n"})
	commit := func(status model.Status) Outcome {
		reg, err := checks.Build(append(checks.Catalog(), limitCheck{status: status}))
		if err != nil {
			t.Fatal(err)
		}
		l := &loading{state: t.TempDir()}
		return RunPreCommit(t.Context(), Dependencies{
			Dir: r.Root, Environ: gittest.Isolated(), Now: time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC),
			Load: l.load, Registry: reg, State: l.state, Stdin: strings.NewReader(""),
		})
	}
	limit := commit(statedLimit("Makefile:3: $(shell) in the prerequisites of ci"))
	want := "golaw pre-commit: fixture.stated-limit left to the pipeline: Makefile:3: $(shell) in the prerequisites of ci: outside the declared subset, a stated limit; docs/rules.md names the rewrite"
	if limit.Code != codeClean || !strings.HasPrefix(limit.Lines[0], "verdict clean") || !slices.Contains(limit.Lines, want) {
		t.Fatalf("a staged stated limit = %+v", limit)
	}
	if other := commit(model.Unmeasured("read Makefile: path unreadable")); other.Code != codeFailure {
		t.Errorf("an unreadable configuration = %+v, want the failure code", other)
	}
}
