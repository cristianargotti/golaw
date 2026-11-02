package hook

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestTheRegenerationLinesComeAheadOfTheLinterLines(t *testing.T) {
	t.Parallel()
	f := fed{
		findings: []model.Finding{{Check: "lint.ruff", Rule: "ruff/S105", Path: "a.py", Line: 3, Message: "Possible hardcoded secret"}},
		regen:    RegenReport{Lines: []string{"hole open a.go:4 a.read do it"}, Person: []string{"lacuna aberta a.go:4 a.read do it"}},
	}
	got := f.apply(Result{Outcome: Allow}, model.DefaultLanguage)
	if !slices.Equal(got.Lines, []string{"hole open a.go:4 a.read do it", "a.py:3 ruff/S105 Possible hardcoded secret"}) {
		t.Fatalf("lines = %q", got.Lines)
	}
	if got.Human == "" {
		t.Errorf("the person reads nothing")
	}
	if empty := (fed{}).apply(Result{Outcome: Allow}, model.DefaultLanguage); len(empty.Lines) != 0 || empty.Human != "" {
		t.Errorf("an empty feedback changed the result: %+v", empty)
	}
	if !(RegenReport{}).empty() || (RegenReport{Notices: []string{"x"}}).empty() {
		t.Errorf("empty() misreads a report")
	}
}
