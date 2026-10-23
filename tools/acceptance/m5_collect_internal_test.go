package main

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// m5Sample is a catalog of three cases: one passes, one fails and one
// passes with its own detail; the first two record the home they ran in.
func m5Sample(homes *[]string) m5Catalog {
	return m5Catalog{category: "m5-sample", cases: []m5Case{
		{name: "pass", run: func(s *suite, _ context.Context) error {
			*homes = append(*homes, s.home)
			return nil
		}},
		{name: "fail", run: func(s *suite, _ context.Context) error {
			*homes = append(*homes, s.home)
			return ErrAcceptance
		}},
		{
			name:   "detail",
			run:    func(*suite, context.Context) error { return nil },
			detail: func(root string) string { return "named from " + filepath.Base(root) },
		},
	}}
}

func TestM5CollectCatalogsRunsEveryCaseInItsOwnHome(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	var homes []string
	rows, evidence := s.m5CollectCatalogs(t.Context(), []m5Catalog{m5Sample(&homes)})
	keys := make([]string, 0, len(rows))
	for _, item := range rows {
		keys = append(keys, item.category+"/"+item.name+" "+item.status)
	}
	if !slices.Equal(keys, []string{"m5-sample/pass PASS", "m5-sample/fail FAIL", "m5-sample/detail PASS"}) {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].detail != m5Verified || rows[1].detail != ErrAcceptance.Error() || rows[2].detail != "named from "+filepath.Base(s.root) {
		t.Fatalf("details = %+v", rows)
	}
	if len(homes) != 2 || homes[0] == homes[1] || homes[0] == s.home {
		t.Fatalf("homes = %q, suite home %q", homes, s.home)
	}
	if len(evidence) != 3 {
		t.Fatalf("evidence = %+v", evidence)
	}
	for key, got := range evidence {
		if got.Agent != binaryName || got.Version != m5CandidateVersion || got.Binary != s.digest {
			t.Fatalf("%s evidence = %+v", key, got)
		}
	}
}

// TestM5CatalogsAreWellFormed holds every category the row tasks add: a
// key of category and name is unique and every case runs something.
func TestM5CatalogsAreWellFormed(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, catalog := range m5Catalogs() {
		for _, c := range catalog.cases {
			key := catalog.category + "/" + c.name
			if seen[key] || c.run == nil || !strings.HasPrefix(catalog.category, "m5-") {
				t.Fatalf("case %s is repeated, has no run or sits outside the M5 categories", key)
			}
			seen[key] = true
		}
	}
	if got := measuredM5("m5-sample", "red", ErrAcceptance); got.status != statusFail || got.detail != ErrAcceptance.Error() || !got.required {
		t.Fatalf("a failing row = %+v", got)
	}
}

func TestM5PreparationFailureIsTheOneRequiredRow(t *testing.T) {
	t.Parallel()
	failing := newSuite(testRoot(t), t.TempDir(), func(context.Context, command) result { return result{err: ErrAcceptance} },
		func(name string) (string, error) { return name, nil })
	var out bytes.Buffer
	if code := failing.runM5(t.Context(), &out); code != 1 ||
		!strings.Contains(out.String(), "m5-preparation\tgolaw\t") || !strings.Contains(out.String(), "\tFAIL\trequired\tcandidate\t") {
		t.Fatalf("a failed M5 candidate = %d\n%s", code, out.String())
	}
}

func TestRunArgsRoutesTheM5Flag(t *testing.T) {
	t.Parallel()
	called := false
	failing := func(context.Context, command) result {
		called = true
		return result{err: ErrAcceptance}
	}
	lookup := func(name string) (string, error) { return filepath.Join(t.TempDir(), name), nil }
	var out bytes.Buffer
	if code := runArgs(&out, failing, lookup, []string{"-m5", "unexpected"}); code != 1 || called {
		t.Fatalf("an extra argument prepared the candidate: %d %v", code, called)
	}
	if code := runArgs(&out, failing, lookup, []string{"-m5"}); code != 1 ||
		!strings.Contains(out.String(), "m5-preparation\tgolaw\t") || !strings.Contains(out.String(), "\tFAIL\trequired\tcandidate\t") {
		t.Fatalf("a failed M5 candidate = %d\n%s", code, out.String())
	}
	only, off := true, false
	if code, routed := (m5Flags{only: &only}).route(t.Context(), &out, nil, true); routed || code != 0 {
		t.Fatal("a release run routed to the M5 rows alone")
	}
	if _, routed := (m5Flags{only: &off}).route(t.Context(), &out, nil, false); routed {
		t.Fatal("a run without -m5 routed to the M5 rows")
	}
}

func TestM5ScopeIsAHomeOfItsOwn(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	first, second := s.m5Scope("m5-sample", "first"), s.m5Scope("m5-sample", "second")
	if first.home == second.home || first.home == s.home || first.binary != s.binary || first.digest != s.digest {
		t.Fatalf("scopes %q and %q over %q", first.home, second.home, s.home)
	}
	if got := environmentValue(first.env, "HOME"); got != first.home {
		t.Fatalf("HOME = %q, want %q", got, first.home)
	}
}

// TestWithM2BoundBoundsOnlyAnUnboundedCommand holds the bound every M5 step
// runs under: a command without a timeout takes the M2 one, and a command
// with its own keeps it.
func TestWithM2BoundBoundsOnlyAnUnboundedCommand(t *testing.T) {
	t.Parallel()
	var seen []time.Duration
	bounded := withM2Bound(func(_ context.Context, c command) result {
		seen = append(seen, c.timeout)
		return result{}
	})
	bounded(t.Context(), command{})
	bounded(t.Context(), command{timeout: time.Second})
	if !slices.Equal(seen, []time.Duration{m2ProcessTimeout, time.Second}) {
		t.Fatalf("timeouts = %v", seen)
	}
}
