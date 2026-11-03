package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func runContext(t *testing.T, a testApp, args ...string) error {
	t.Helper()
	cmd := a.contextCommand()
	cmd.SetArgs(args)
	cmd.SetOut(a.stdout)
	return cmd.ExecuteContext(t.Context()) //nolint:wrapcheck // the test reads the command's own error
}

func TestContextPrintsTheShapesOfAnArea(t *testing.T) {
	t.Parallel()
	a, root := mapApp(t, proof.Policy())
	if err := runContext(t, a, "growth", root); err != nil {
		t.Fatal(err)
	}
	out := a.stdout.String()
	want := "growth.Ledger internal/application/growth/ports.go:10\n  ReadPlan(ctx context.Context, account AccountID) (Plan, error)\n"
	if !strings.HasPrefix(out, "golaw context growth: ") || !strings.Contains(out, want) || !strings.Contains(out, "\n## tables\nplans db/migrations/0001_plans.sql:") {
		t.Fatalf("out =\n%s", out)
	}
	a.stdout.Reset()
	if err := runContext(t, a, "--json", "growth", root); err != nil || !strings.HasPrefix(a.stdout.String(), "{") {
		t.Fatalf("json = %q, %v", a.stdout.String(), err)
	}
}

// Red rows: an unknown area names the areas there are, an unenrolled
// repository and a budget below one are usage errors.
func TestContextRefusesAnUnknownAreaOutsideGolawAndABadBudget(t *testing.T) {
	t.Parallel()
	a, root := mapApp(t, proof.Policy())
	err := runContext(t, a, "billing", root)
	if !errors.Is(err, model.ErrUsage) || !strings.Contains(err.Error(), "has no area billing: the areas are growth") {
		t.Fatalf("unknown area: %v", err)
	}
	unenrolled := *proof.Policy()
	unenrolled.Enrollment = model.Enrollment{State: model.EnrollmentNone}
	b, root := mapApp(t, &unenrolled)
	if err := runContext(t, b, "growth", root); !errors.Is(err, model.ErrUsage) || !strings.Contains(err.Error(), "run golaw init") {
		t.Fatalf("unenrolled: %v", err)
	}
	c, root := mapApp(t, proof.Policy())
	if err := runContext(t, c, "--budget", "0", "growth", root); !errors.Is(err, model.ErrUsage) {
		t.Fatalf("budget 0: %v", err)
	}
	if err := runContext(t, c, "growth", root+"/missing"); err == nil {
		t.Fatal("a missing root answered no error")
	}
}
