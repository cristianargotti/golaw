package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// m6Sample is a catalog of three cases: one passes, one fails and one
// needs the surface tools; the first two record the home they ran in.
func m6Sample(homes *[]string) m6Catalog {
	return m6Catalog{category: "m6-sample", cases: []m6Case{
		{name: "pass", run: func(s *m6Suite, _ context.Context) error {
			*homes = append(*homes, s.home)
			return nil
		}},
		{name: "fail", run: func(s *m6Suite, _ context.Context) error {
			*homes = append(*homes, s.home)
			return ErrAcceptance
		}},
		{name: "tools", tools: true, run: func(*m6Suite, context.Context) error { return nil }},
	}}
}

func TestM6CollectRunsEveryCaseInItsOwnHome(t *testing.T) {
	t.Parallel()
	s := newM6Suite(m2TestSuite(t))
	cause := fmt.Errorf("node is not on PATH: %w", ErrAcceptance)
	s.shared.tried, s.shared.err = true, cause
	var homes []string
	rows, evidence := s.m6CollectCatalogs(t.Context(), []m6Catalog{m6Sample(&homes)})
	keys := make([]string, 0, len(rows))
	for _, item := range rows {
		keys = append(keys, item.category+"/"+item.name+" "+item.status)
	}
	if !slices.Equal(keys, []string{"m6-sample/pass PASS", "m6-sample/fail FAIL", "m6-sample/tools SKIP"}) {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].detail != m6Verified || rows[1].detail != ErrAcceptance.Error() ||
		!rows[2].required || rows[2].detail != m6ToolsSkipped+cause.Error() {
		t.Fatalf("details = %+v", rows)
	}
	if len(homes) != 2 || homes[0] == homes[1] || homes[0] == s.home {
		t.Fatalf("homes = %q, suite home %q", homes, s.home)
	}
	if len(evidence) != 3 {
		t.Fatalf("evidence = %+v", evidence)
	}
	for key, got := range evidence {
		if got.Agent != binaryName || got.Version != m6CandidateVersion || got.Binary != s.digest {
			t.Fatalf("%s evidence = %+v", key, got)
		}
	}
}

// TestM6CatalogsAreWellFormed holds every category the row tasks add: a
// key of category and name is unique and every case runs something.
func TestM6CatalogsAreWellFormed(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, catalog := range m6Catalogs() {
		if !slices.Equal(m6Names(catalog.cases), m6Names(slices.Clone(catalog.cases))) || !strings.HasPrefix(catalog.category, "m6-") {
			t.Fatalf("catalog %s sits outside the M6 categories", catalog.category)
		}
		for _, c := range catalog.cases {
			key := catalog.category + "/" + c.name
			if seen[key] || c.run == nil {
				t.Fatalf("case %s is repeated or has no run", key)
			}
			seen[key] = true
		}
	}
	if got := measuredM6("m6-sample", "red", ErrAcceptance); got.status != statusFail || got.detail != ErrAcceptance.Error() || !got.required {
		t.Fatalf("a failing row = %+v", got)
	}
}

func TestRunArgsRoutesTheM6Flag(t *testing.T) {
	t.Parallel()
	called := false
	failing := func(context.Context, command) result {
		called = true
		return result{err: ErrAcceptance}
	}
	lookup := func(name string) (string, error) { return filepath.Join(t.TempDir(), name), nil }
	var out bytes.Buffer
	if code := runArgs(&out, failing, lookup, []string{"-m6", "unexpected"}); code != 1 || called {
		t.Fatalf("an extra argument prepared the candidate: %d %v", code, called)
	}
	if code := runArgs(&out, failing, lookup, []string{"-m6"}); code != 1 ||
		!strings.Contains(out.String(), "m6-preparation\tgolaw\t") || !strings.Contains(out.String(), "\tFAIL\trequired\tcandidate\t") {
		t.Fatalf("a failed M6 candidate = %d\n%s", code, out.String())
	}
	only, off := true, false
	if code, routed := (m6Flags{only: &only}).route(t.Context(), &out, nil, true); routed || code != 0 {
		t.Fatal("a release run routed to the M6 rows alone")
	}
	if _, routed := (m6Flags{only: &off}).route(t.Context(), &out, nil, false); routed {
		t.Fatal("a run without -m6 routed to the M6 rows")
	}
}
