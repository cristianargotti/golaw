package main

import (
	"context"
	"flag"
	"io"
)

// The M5 rows of section 11: the version of the candidate every row
// builds (D21), the preparation row a failed build answers with, and the
// detail of a passing row.
const (
	m5CandidateVersion = "0.5.0"
	m5Preparation      = "m5-preparation"
	m5Verified         = "verified against the M5 candidate"
)

// m5Case is one row: its name, the function that runs it in the scope of
// its own home, and the detail a passing row carries when the default
// would say less than the row proves.
type m5Case struct {
	name   string
	run    func(*suite, context.Context) error
	detail func(root string) string
}

// m5Catalog is one category of M5 rows in report order.
type m5Catalog struct {
	category string
	cases    []m5Case
}

// m5Catalogs are the categories of the M5 rows in report order.
func m5Catalogs() []m5Catalog {
	return []m5Catalog{}
}

func (s *suite) collectM5(ctx context.Context) ([]row, map[string]rowEvidence) {
	return s.m5CollectCatalogs(ctx, m5Catalogs())
}

// m5CollectCatalogs runs every case in the scope of its own home and keys
// the evidence of each row by category and name, as the M2 to M4 rows are
// keyed.
func (s *suite) m5CollectCatalogs(ctx context.Context, catalogs []m5Catalog) ([]row, map[string]rowEvidence) {
	var rows []row
	evidence := make(map[string]rowEvidence)
	for _, catalog := range catalogs {
		for _, c := range catalog.cases {
			item := measuredM5(catalog.category, c.name, c.run(s.m5Scope(catalog.category, c.name), ctx))
			if item.status == statusPass && c.detail != nil {
				item.detail = c.detail(s.root)
			}
			rows = append(rows, item)
			evidence[item.category+"/"+item.name] = s.m5Evidence()
		}
	}
	return rows, evidence
}

// m5Evidence identifies every M5 row by the candidate it ran.
func (s *suite) m5Evidence() rowEvidence {
	return rowEvidence{Agent: binaryName, Version: m5CandidateVersion, Binary: s.digest}
}

// measuredM5 is measured with the detail of a passing M5 row.
func measuredM5(category, name string, err error) row {
	got := measured(category, name, err)
	if err == nil {
		got.detail = m5Verified
	}
	return got
}

// m5AcceptanceRows gives every command the M2 bound when it has none,
// prepares the candidate and collects the rows.
func (s *suite) m5AcceptanceRows(ctx context.Context) ([]row, map[string]rowEvidence) {
	s.execute = withM2Bound(s.execute)
	if err := s.prepareM5(ctx); err != nil {
		item := measuredM5(m5Preparation, candidateRow, err)
		return []row{item}, map[string]rowEvidence{item.category + "/" + item.name: s.m5Evidence()}
	}
	return s.collectM5(ctx)
}

// withM2Bound bounds a command that names no timeout of its own by the
// M2 process bound, so no M5 step can hang the run.
func withM2Bound(execute executor) executor {
	return func(ctx context.Context, c command) result {
		if c.timeout == 0 {
			c.timeout = m2ProcessTimeout
		}
		return execute(ctx, c)
	}
}

// runM5 reports the M5 rows alone.
func (s *suite) runM5(ctx context.Context, out io.Writer) int {
	rows, evidence := s.m5AcceptanceRows(ctx)
	return m2ReportCode(reportEvidence(out, rows, "", s.digest, evidence))
}

// m5Flags are the flags of the M5 runs: -m5 reports the M5 rows alone.
type m5Flags struct {
	only *bool
}

func m5FlagSet(flags *flag.FlagSet) m5Flags {
	return m5Flags{only: flags.Bool("m5", false, "require M5 functional evidence rows")}
}

// route runs the M5 rows when -m5 selects them; a release run keeps every
// row, so it never routes here.
func (f m5Flags) route(ctx context.Context, out io.Writer, s *suite, release bool) (int, bool) {
	if release || !*f.only {
		return 0, false
	}
	return s.runM5(ctx, out), true
}
