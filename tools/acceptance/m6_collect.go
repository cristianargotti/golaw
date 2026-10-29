package main

import (
	"context"
	"flag"
	"io"
	"path/filepath"
	"time"
)

// The M6 rows of M6.10: the version of the candidate every row builds, the
// preparation row a failed build answers with, the detail of a passing row
// and the prefix of a row skipped because the surface tools are absent.
const (
	m6CandidateVersion = "0.6.0"
	m6Preparation      = "m6-preparation"
	m6Verified         = "verified against the M6 candidate"
	m6ToolsSkipped     = "surface tools unavailable: "
)

// m6Shared is what every M6 case of one run shares: the directory of the
// run, the data home the surface tools install into, the clock the budget
// rows read, and the memo of the one tool install, so a failed download is
// reported by every row that needs it instead of retried by each.
type m6Shared struct {
	base  string
	data  string
	now   func() time.Time
	tried bool
	err   error
}

// m6Suite is a suite with the state the M6 cases of one run share.
type m6Suite struct {
	*suite

	shared *m6Shared
}

// m6Case is one row: its name, whether it needs the surface tools, and the
// function that runs it in the scope of its own home.
type m6Case struct {
	name  string
	tools bool
	run   func(*m6Suite, context.Context) error
}

// m6Catalog is one category of M6 rows in report order.
type m6Catalog struct {
	category string
	cases    []m6Case
}

// m6Catalogs are the categories of the M6 rows in report order; each row
// task appends its own category.
func m6Catalogs() []m6Catalog {
	return []m6Catalog{
		{category: m6CoverageCategory, cases: m6CoverageCases()},
		{category: m6DenialCategory, cases: m6DenialCases()},
		{category: m6SetupCategory, cases: m6SetupCases()},
	}
}

// m6Names are the names of cases in order.
func m6Names(cases []m6Case) []string {
	names := make([]string, 0, len(cases))
	for _, c := range cases {
		names = append(names, c.name)
	}
	return names
}

func newM6Suite(s *suite) *m6Suite {
	base := filepath.Join(s.base, "m6")
	clock := time.Now //nolint:forbidigo // A main package is a composition root and reads the clock, as cmd does.
	return &m6Suite{suite: s, shared: &m6Shared{base: base, data: filepath.Join(base, m6DataDir), now: clock}}
}

// m6CollectCatalogs runs every case and keys the evidence of each row by
// category and name, as the M2 to M5 rows are keyed.
func (s *m6Suite) m6CollectCatalogs(ctx context.Context, catalogs []m6Catalog) ([]row, map[string]rowEvidence) {
	var rows []row
	evidence := make(map[string]rowEvidence)
	for _, catalog := range catalogs {
		for _, c := range catalog.cases {
			item := s.m6Row(ctx, catalog.category, c)
			rows = append(rows, item)
			evidence[item.category+"/"+item.name] = s.m6Evidence()
		}
	}
	return rows, evidence
}

// m6Row runs one case in its own home; a case that needs the surface tools
// is a required SKIP with the cause when they could not be installed, so
// the lane records it and a release run fails on it.
func (s *m6Suite) m6Row(ctx context.Context, category string, c m6Case) row {
	scope := s.m6Scope(category, c.name)
	if c.tools {
		if err := s.m6Tools(ctx); err != nil {
			return row{category, c.name, statusSkip, m6ToolsSkipped + err.Error(), true}
		}
	}
	return measuredM6(category, c.name, c.run(scope, ctx))
}

// m6Evidence identifies every M6 row by the candidate it ran.
func (s *suite) m6Evidence() rowEvidence {
	return rowEvidence{Agent: binaryName, Version: m6CandidateVersion, Binary: s.digest}
}

// measuredM6 is measured with the detail of a passing M6 row.
func measuredM6(category, name string, err error) row {
	got := measured(category, name, err)
	if err == nil {
		got.detail = m6Verified
	}
	return got
}

// m6AcceptanceRows gives every command the M2 bound when it has none,
// prepares the candidate and collects the rows.
func (s *suite) m6AcceptanceRows(ctx context.Context) ([]row, map[string]rowEvidence) {
	s.execute = withM2Bound(s.execute)
	if err := s.prepareM6(ctx); err != nil {
		item := measuredM6(m6Preparation, candidateRow, err)
		return []row{item}, map[string]rowEvidence{item.category + "/" + item.name: s.m6Evidence()}
	}
	return newM6Suite(s).m6CollectCatalogs(ctx, m6Catalogs())
}

// runM6 reports the M6 rows alone.
func (s *suite) runM6(ctx context.Context, out io.Writer) int {
	rows, evidence := s.m6AcceptanceRows(ctx)
	return m2ReportCode(reportEvidence(out, rows, "", s.digest, evidence))
}

// m6Flags are the flags of the M6 runs: -m6 reports the M6 rows alone.
type m6Flags struct {
	only *bool
}

func m6FlagSet(flags *flag.FlagSet) m6Flags {
	return m6Flags{only: flags.Bool("m6", false, "require M6 functional evidence rows")}
}

// route runs the M6 rows when -m6 selects them; a release run keeps every
// row, so it never routes here.
func (f m6Flags) route(ctx context.Context, out io.Writer, s *suite, release bool) (int, bool) {
	if release || !*f.only {
		return 0, false
	}
	return s.runM6(ctx, out), true
}
