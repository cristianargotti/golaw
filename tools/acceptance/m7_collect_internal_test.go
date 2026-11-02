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

// m7Sample is a catalog of four cases: one passes with a note, one fails,
// one needs the generators and one skips with its own cause; the first two
// record the home they ran in.
func m7Sample(homes *[]string) m7Catalog {
	return m7Catalog{category: "m7-sample", cases: []m7Case{
		{name: "pass", run: func(s *m7Suite, _ context.Context) error {
			*homes = append(*homes, s.home)
			s.m7Note("rendering digest 00ff")
			return nil
		}},
		{name: "fail", run: func(s *m7Suite, _ context.Context) error {
			*homes = append(*homes, s.home)
			return ErrAcceptance
		}},
		{name: "generators", gen: true, run: func(*m7Suite, context.Context) error { return nil }},
		{name: "skipped", run: func(*m7Suite, context.Context) error {
			return m7Skip("golangci-lint 2.14.0 not found: run golaw setup --gen")
		}},
	}}
}

func TestM7CollectRunsEveryCaseInItsOwnHome(t *testing.T) {
	t.Parallel()
	s := newM7Suite(m2TestSuite(t))
	cause := fmt.Errorf("sqlc did not build: %w", ErrAcceptance)
	s.gen.tried, s.gen.err = true, cause
	var homes []string
	rows, evidence := s.m7CollectCatalogs(t.Context(), []m7Catalog{m7Sample(&homes)})
	keys := make([]string, 0, len(rows))
	for _, item := range rows {
		keys = append(keys, item.category+"/"+item.name+" "+item.status)
	}
	want := []string{"m7-sample/pass PASS", "m7-sample/fail FAIL", "m7-sample/generators SKIP", "m7-sample/skipped SKIP"}
	if !slices.Equal(keys, want) {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].detail != "rendering digest 00ff" || rows[1].detail != ErrAcceptance.Error() ||
		!rows[2].required || rows[2].detail != m7GenSkipped+cause.Error() ||
		!rows[3].required || rows[3].detail != "golangci-lint 2.14.0 not found: run golaw setup --gen" {
		t.Fatalf("details = %+v", rows)
	}
	if len(homes) != 2 || homes[0] == homes[1] || homes[0] == s.home {
		t.Fatalf("homes = %q, suite home %q", homes, s.home)
	}
	if len(evidence) != len(want) {
		t.Fatalf("evidence = %+v", evidence)
	}
	for key, got := range evidence {
		if got.Agent != binaryName || got.Version != m7CandidateVersion || got.Binary != s.digest {
			t.Fatalf("%s evidence = %+v", key, got)
		}
	}
}

// TestM7NoteDoesNotLeakIntoTheNextRow: a note belongs to the row that
// wrote it; the next passing row reads the plain verified detail.
func TestM7NoteDoesNotLeakIntoTheNextRow(t *testing.T) {
	t.Parallel()
	s := newM7Suite(m2TestSuite(t))
	noted := m7Case{name: "noted", run: func(s *m7Suite, _ context.Context) error {
		s.m7Note("digest abc")
		return nil
	}}
	plain := m7Case{name: "plain", run: func(*m7Suite, context.Context) error { return nil }}
	if got := s.m7Row(t.Context(), "m7-sample", noted); got.detail != "digest abc" {
		t.Fatalf("noted row = %+v", got)
	}
	if got := s.m7Row(t.Context(), "m7-sample", plain); got.detail != m7Verified {
		t.Fatalf("plain row = %+v", got)
	}
}

// TestM7CatalogsAreWellFormed holds every category the row tasks add: a
// key of category and name is unique and every case runs something.
func TestM7CatalogsAreWellFormed(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for _, catalog := range m7Catalogs() {
		if !strings.HasPrefix(catalog.category, "m7-") || len(m7Names(catalog.cases)) != len(catalog.cases) {
			t.Fatalf("catalog %s sits outside the M7 categories", catalog.category)
		}
		for _, c := range catalog.cases {
			key := catalog.category + "/" + c.name
			if seen[key] || c.run == nil {
				t.Fatalf("case %s is repeated or has no run", key)
			}
			seen[key] = true
		}
	}
	if got := measuredM7("m7-sample", "red", ErrAcceptance); got.status != statusFail || got.detail != ErrAcceptance.Error() || !got.required {
		t.Fatalf("a failing row = %+v", got)
	}
}

func TestRunArgsRoutesTheM7Flag(t *testing.T) {
	t.Parallel()
	called := false
	failing := func(context.Context, command) result {
		called = true
		return result{err: ErrAcceptance}
	}
	lookup := func(name string) (string, error) { return filepath.Join(t.TempDir(), name), nil }
	var out bytes.Buffer
	if code := runArgs(&out, failing, lookup, []string{"-m7", "unexpected"}); code != 1 || called {
		t.Fatalf("an extra argument prepared the candidate: %d %v", code, called)
	}
	if code := runArgs(&out, failing, lookup, []string{"-m7"}); code != 1 ||
		!strings.Contains(out.String(), "m7-preparation\tgolaw\t") || !strings.Contains(out.String(), "\tFAIL\trequired\tcandidate\t") {
		t.Fatalf("a failed M7 candidate = %d\n%s", code, out.String())
	}
	only, off := true, false
	if code, routed := (m7Flags{only: &only}).route(t.Context(), &out, nil, true); routed || code != 0 {
		t.Fatal("a release run routed to the M7 rows alone")
	}
	if _, routed := (m7Flags{only: &off}).route(t.Context(), &out, nil, false); routed {
		t.Fatal("a run without -m7 routed to the M7 rows")
	}
	later := laterFlags{m6: m6Flags{only: &off}, m7: m7Flags{only: &off}}
	if _, routed := later.route(t.Context(), &out, nil, false); routed {
		t.Fatal("a run without -m6 and -m7 routed to a later milestone")
	}
}
