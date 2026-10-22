package model_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestOutsideSubsetIsAStatedLimitNeverClean pins the status a policy check
// answers for a construct outside the subset its reader declares: Unmeasured,
// so the pipeline never answers clean over it, with a reason that names the
// construct and ends with the sentinel the rungs recognize.
func TestOutsideSubsetIsAStatedLimitNeverClean(t *testing.T) {
	t.Parallel()
	reader := fmt.Errorf("%s: %w", "Makefile:12: $(shell) in the prerequisites of ci", model.ErrOutsideSubset)
	status := model.Unmeasured(reader.Error())
	if status.Kind != model.StatusUnmeasured {
		t.Fatalf("kind = %s, want unmeasured", status.Kind)
	}
	want := "Makefile:12: $(shell) in the prerequisites of ci: outside the declared subset, a stated limit; docs/rules.md names the rewrite"
	if status.Reason != want {
		t.Fatalf("reason = %q, want %q", status.Reason, want)
	}
	if !model.IsOutsideSubset(status) {
		t.Fatal("IsOutsideSubset misses the stated limit")
	}
	if !errors.Is(fmt.Errorf("read Makefile: %w", model.ErrOutsideSubset), model.ErrOutsideSubset) {
		t.Fatal("ErrOutsideSubset must match through wrapping")
	}
}

func TestOnlyAStatedLimitIsOutsideTheSubset(t *testing.T) {
	t.Parallel()
	for name, status := range map[string]model.Status{
		"unreadable":     model.Unmeasured("read Makefile: path unreadable"),
		"read bound":     model.Unmeasured("read Makefile: " + model.ErrTooLarge.Error()),
		"not applicable": model.NotApplicable("x: " + model.ErrOutsideSubset.Error()),
		"measured":       model.Measured(),
		"limit inside":   model.Unmeasured(model.ErrOutsideSubset.Error() + " in Makefile"),
	} {
		if model.IsOutsideSubset(status) {
			t.Errorf("%s: IsOutsideSubset = true", name)
		}
	}
}

// TestCoreDefaultsHoldTheLayoutDefaults pins the layout a policy that names
// none holds: the main package at the root and the decision records under
// docs/adr (decision 5 of the specification).
func TestCoreDefaultsHoldTheLayoutDefaults(t *testing.T) {
	t.Parallel()
	got := model.CoreDefaults().Layout
	want := model.Layout{Main: model.DefaultMain, Decisions: model.DefaultDecisions}
	if got != want || want.Main != "root" || want.Decisions != "docs/adr" {
		t.Fatalf("layout = %+v, want %+v", got, want)
	}
}
