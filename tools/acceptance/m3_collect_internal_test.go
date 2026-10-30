package main

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestM3CollectorSkipsTheRowsOfAnotherSystem(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	calls := 0
	catalog := m3Catalog{
		category: "m3-sample", names: []string{"a"}, only: "linux",
		run: func(*suite, context.Context, string) error {
			calls++
			return nil
		},
	}
	s.m2OS = "darwin"
	if got := s.m3CollectedRow(t.Context(), catalog, "a"); got.status != statusSkip || got.required || calls != 0 ||
		!strings.Contains(got.detail, "linux") {
		t.Fatalf("another system = %+v, %d calls", got, calls)
	}
	s.m2OS = "linux"
	if got := s.m3CollectedRow(t.Context(), catalog, "a"); got.status != statusPass || !got.required || got.detail != m3Verified || calls != 1 {
		t.Fatalf("its system = %+v, %d calls", got, calls)
	}
	catalog.run = func(*suite, context.Context, string) error { return ErrAcceptance }
	if got := s.m3CollectedRow(t.Context(), catalog, "a"); got.status != statusFail || !got.required {
		t.Fatalf("a failing row = %+v", got)
	}
}

func TestM3CollectCatalogsKeysEveryRowAndItsEvidence(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	s.m2OS, s.digest = "linux", strings.Repeat("a", 64)
	pass := func(*suite, context.Context, string) error { return nil }
	rows, evidence := s.m3CollectCatalogs(t.Context(), []m3Catalog{
		{category: "m3-first", names: []string{"a", "b"}, run: pass},
		{category: "m3-second", names: []string{"c"}, only: "windows", run: pass},
	})
	keys := make([]string, 0, len(rows))
	for _, item := range rows {
		keys = append(keys, item.category+"/"+item.name)
	}
	if !slices.Equal(keys, []string{"m3-first/a", "m3-first/b", "m3-second/c"}) || rows[2].status != statusSkip {
		t.Fatalf("rows = %+v", rows)
	}
	for _, key := range keys {
		if identity := evidence[key]; identity.Agent != binaryName || identity.Version != m3CandidateVersion || identity.Binary != s.digest {
			t.Fatalf("%s evidence = %+v", key, identity)
		}
	}
}

func TestM3PolicyBaselinePassesItsOwnCI(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		setup int
		code  int
		fail  bool
		steps []string
	}{
		{"near make ci passes", 0, 0, false, []string{"origin", "surface", "make"}},
		{"red make ci fails", 0, 2, true, []string{"origin", "surface", "make"}},
		{"red the surface tools do not install", 2, 0, true, []string{"origin", "surface"}},
	} {
		s := m2TestSuite(t)
		s.m2.Make = "/simulated/make"
		var calls [][]string
		var order []string
		service := m3ServiceAnswer(t, s, &calls)
		s.execute = newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
			switch {
			case c.program == s.m2.Make:
				order = append(order, "make")
				return m2Reply{got: result{code: tc.code, stdout: "make ci\n"}, target: true}, true
			case c.program == s.git && slices.Equal(c.args, m3CIDefaultBranch()):
				order = append(order, "origin")
				return m2Reply{got: result{}}, true
			case c.program == s.binary && slices.Equal(c.args, m3SurfaceSetup()):
				order = append(order, "surface")
				return m2Reply{got: result{code: tc.setup}, target: true}, true
			}
			return service(ctx, c)
		}).Execute
		if err := s.policyM3(t.Context(), m3Baseline); (err != nil) != tc.fail {
			t.Fatalf("%s: baseline = %v", tc.name, err)
		}
		// A CI checkout holds the default branch, which coverage-new reads,
		// and the generated CI installs the surface tools before make ci.
		if !slices.Equal(order, tc.steps) {
			t.Fatalf("%s: steps = %q, want %q", tc.name, order, tc.steps)
		}
	}
	s := m2TestSuite(t)
	if err := s.policyM3(t.Context(), "absent"); err == nil {
		t.Fatal("an unknown policy case must fail")
	}
	if !slices.Contains(m3PolicyNames(), m3Baseline) {
		t.Fatal("the baseline row is missing")
	}
}
