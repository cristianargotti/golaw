package bootstrap_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
)

func TestResolveLayoutPreservesReviewedChoices(t *testing.T) {
	t.Parallel()
	main, decisions := bootstrap.MainRoot, "docs/decisions"
	root := repo(t, "go.mod", "cmd", "docs/adr", "docs/decisions")
	got, err := bootstrap.ResolveLayout(root, &main, &decisions)
	want := bootstrap.Layout{Main: main, Decisions: decisions, Go: true}
	if err != nil || got != want {
		t.Fatalf("layout = %+v, %v, want %+v", got, err, want)
	}
	if _, err := bootstrap.Detect(root); !errors.Is(err, bootstrap.ErrDecisions) {
		t.Fatalf("unreviewed detection error = %v", err)
	}
}

func TestResolveLayoutDetectsOnlyAbsentChoices(t *testing.T) {
	t.Parallel()
	main, decisions := bootstrap.MainRoot, "docs/adr"
	for _, tc := range []struct {
		name, main, decisions string
		entries               []string
		reviewedMain          *string
		reviewedDecisions     *string
	}{
		{
			name: "main from checkout", main: bootstrap.MainCmd, decisions: decisions,
			entries: []string{"cmd", "docs/adr", "docs/decisions"}, reviewedDecisions: &decisions,
		},
		{
			name: "decisions from checkout", main: main, decisions: "docs/decisions",
			entries: []string{"cmd", "docs/decisions"}, reviewedMain: &main,
		},
		{
			name: "root main wins discovery", main: bootstrap.MainRoot, decisions: decisions,
			entries: []string{"main.go", "cmd", "docs/adr", "docs/decisions"}, reviewedDecisions: &decisions,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.ResolveLayout(repo(t, tc.entries...), tc.reviewedMain, tc.reviewedDecisions)
			want := bootstrap.Layout{Main: tc.main, Decisions: tc.decisions}
			if err != nil || got != want {
				t.Fatalf("layout = %+v, %v, want %+v", got, err, want)
			}
		})
	}
}

func TestResolveLayoutStillRefusesAnAbsentDecisionChoice(t *testing.T) {
	t.Parallel()
	main := bootstrap.MainRoot
	root := repo(t, "docs/adr", "docs/decisions")
	if _, err := bootstrap.ResolveLayout(root, &main, nil); !errors.Is(err, bootstrap.ErrDecisions) {
		t.Fatalf("missing decision choice error = %v", err)
	}
}
