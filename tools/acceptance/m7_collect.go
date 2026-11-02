package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"time"
)

// The M7 rows of M7.13: the version of the candidate every row builds, the
// preparation row a failed build answers with, the detail of a passing row
// and the prefix of a row skipped because the pinned generators are absent.
const (
	m7CandidateVersion = "0.7.0"
	m7Preparation      = "m7-preparation"
	m7Verified         = "verified against the M7 candidate"
	m7GenSkipped       = "generators unavailable: "
)

// m7Shared is what every M7 case of one run shares beyond the M6 state: the
// memo of the one generator build, so a failed build is reported by every
// row that needs it instead of retried by each, and the note a passing row
// leaves as its detail, and the memo of the filled fresh repository three
// rows read, built once per run.
type m7Shared struct {
	tried bool
	err   error
	note  string

	filled      *m7Repo
	filledErr   error
	filledTried bool
}

// m7Suite is an M6 suite with the state the M7 cases of one run share, so
// every M6 helper runs on an M7 scope.
type m7Suite struct {
	*m6Suite

	gen *m7Shared
}

// m7Case is one row: its name, whether it needs the pinned generators, and
// the function that runs it in the scope of its own home.
type m7Case struct {
	name string
	gen  bool
	run  func(*m7Suite, context.Context) error
}

// m7Catalog is one category of M7 rows in report order.
type m7Catalog struct {
	category string
	cases    []m7Case
}

// m7Skipped is a row skipped by a missing pinned tool; a type rather than a
// sentinel because its cause is the text the lane records.
type m7Skipped struct { //nolint:errname // The later row tasks and their tests name the skip by this type.
	cause string
}

func (e *m7Skipped) Error() string { return e.cause }

// m7Skip answers the skip of a row whose pinned tool is absent.
func m7Skip(cause string) error { return &m7Skipped{cause: cause} }

// m7Catalogs are the categories of the M7 rows in report order; each row
// task appends its own category.
func m7Catalogs() []m7Catalog {
	return []m7Catalog{
		{category: m7FreshCategory, cases: m7FreshCases()},
		{category: m7CheckCategory, cases: m7CheckCases()},
		{category: m7DenialCategory, cases: m7DenialCases()},
		{category: m7RegenCategory, cases: m7RegenCases()},
		{category: m7LayoutCategory, cases: m7LayoutCases()},
		{category: m7MapCategory, cases: m7MapCases()},
		{category: m7GoldenCategory, cases: m7GoldenCases()},
	}
}

// m7Names are the names of cases in order.
func m7Names(cases []m7Case) []string {
	names := make([]string, 0, len(cases))
	for _, c := range cases {
		names = append(names, c.name)
	}
	return names
}

func newM7Suite(s *suite) *m7Suite {
	base := filepath.Join(s.base, "m7")
	clock := time.Now //nolint:forbidigo // A main package is a composition root and reads the clock, as cmd does.
	shared := &m6Shared{base: base, data: filepath.Join(base, m6DataDir), now: clock}
	return &m7Suite{m6Suite: &m6Suite{suite: s, shared: shared}, gen: &m7Shared{}}
}

// m7Scope is the isolated home of one M7 case, sharing the run's state.
func (s *m7Suite) m7Scope(category, name string) *m7Suite {
	return &m7Suite{m6Suite: s.m6Scope(category, name), gen: s.gen}
}

// m7Note records the detail the running row reports when it passes.
func (s *m7Suite) m7Note(detail string) {
	s.gen.note = detail
}

// m7CollectCatalogs runs every case and keys the evidence of each row by
// category and name, as the M2 to M6 rows are keyed.
func (s *m7Suite) m7CollectCatalogs(ctx context.Context, catalogs []m7Catalog) ([]row, map[string]rowEvidence) {
	evidence := make(map[string]rowEvidence)
	rows := make([]row, 0, len(catalogs))
	for _, catalog := range catalogs {
		for _, c := range catalog.cases {
			evidence[catalog.category+"/"+c.name] = s.m7Evidence()
			rows = append(rows, s.m7Row(ctx, catalog.category, c))
		}
	}
	return rows, evidence
}

// m7Row runs one case in its own home; a case whose generators could not
// be built, or whose pinned tool is absent, is a required SKIP with the
// cause, so the lane records it and a release run fails on it (A23).
func (s *m7Suite) m7Row(ctx context.Context, category string, c m7Case) row {
	s.gen.note = ""
	scope := s.m7Scope(category, c.name)
	if c.gen {
		if err := s.m7Generators(ctx); err != nil {
			return row{category, c.name, statusSkip, m7GenSkipped + err.Error(), true}
		}
	}
	err := c.run(scope, ctx)
	if skipped, ok := errors.AsType[*m7Skipped](err); ok {
		return row{category, c.name, statusSkip, skipped.cause, true}
	}
	got := measuredM7(category, c.name, err)
	if err == nil && s.gen.note != "" {
		got.detail = s.gen.note
	}
	return got
}

// m7Evidence identifies every M7 row by the candidate it ran.
func (s *suite) m7Evidence() rowEvidence {
	return rowEvidence{Agent: binaryName, Version: m7CandidateVersion, Binary: s.digest}
}

// measuredM7 is measured with the detail of a passing M7 row.
func measuredM7(category, name string, err error) row {
	got := measured(category, name, err)
	if err == nil {
		got.detail = m7Verified
	}
	return got
}

// m7AcceptanceRows gives every command the M2 bound when it has none,
// prepares the candidate and collects the rows.
func (s *suite) m7AcceptanceRows(ctx context.Context) ([]row, map[string]rowEvidence) {
	s.execute = withM2Bound(s.execute)
	err := s.prepareM7(ctx)
	if err == nil {
		return newM7Suite(s).m7CollectCatalogs(ctx, m7Catalogs())
	}
	key := m7Preparation + "/" + candidateRow
	return []row{measuredM7(m7Preparation, candidateRow, err)}, map[string]rowEvidence{key: s.m7Evidence()}
}

// runM7 reports the M7 rows alone.
func (s *suite) runM7(ctx context.Context, out io.Writer) int {
	rows, evidence := s.m7AcceptanceRows(ctx)
	return m2ReportCode(reportEvidence(out, rows, "", s.digest, evidence))
}

// m7Flags are the flags of the M7 runs: -m7 reports the M7 rows alone.
type m7Flags struct {
	only *bool
}

func m7FlagSet(flags *flag.FlagSet) m7Flags {
	return m7Flags{only: flags.Bool("m7", false, "require M7 functional evidence rows")}
}

// route runs the M7 rows when -m7 selects them; a release run keeps every
// row, so it never routes here.
func (f m7Flags) route(ctx context.Context, out io.Writer, s *suite, release bool) (int, bool) {
	if release || !*f.only {
		return 0, false
	}
	return s.runM7(ctx, out), true
}

// laterFlags are the flags of the milestones after M5, routed in order, so
// runArgs keeps one route line for all of them.
type laterFlags struct {
	m6 m6Flags
	m7 m7Flags
}

func laterFlagSet(flags *flag.FlagSet) laterFlags {
	return laterFlags{m6: m6FlagSet(flags), m7: m7FlagSet(flags)}
}

// route answers the first milestone route that runs.
func (f laterFlags) route(ctx context.Context, out io.Writer, s *suite, release bool) (int, bool) {
	if code, routed := f.m6.route(ctx, out, s, release); routed {
		return code, true
	}
	return f.m7.route(ctx, out, s, release)
}
