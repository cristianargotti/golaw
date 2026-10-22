package main

import (
	"context"
	"io"
)

// candidateRow names the one row a run answers when its candidate cannot
// be prepared.
const candidateRow = "candidate"

// m3Catalog is one family of M3 rows: its category, its case names, the
// one operating system it is required on when the generated CI pins one,
// and the function that runs a case in the scope of its own home.
type m3Catalog struct {
	category string
	names    []string
	only     string
	run      func(*suite, context.Context, string) error
}

func m3Catalogs() []m3Catalog {
	return []m3Catalog{
		{category: m3PolicyCategory, names: m3PolicyNames(), run: (*suite).policyM3},
		{category: m3TemplateCategory, names: m3TemplateNames(), only: m3ReplayOS, run: (*suite).templateM3},
	}
}

func (s *suite) collectM3(ctx context.Context) ([]row, map[string]rowEvidence) {
	return s.m3CollectCatalogs(ctx, m3Catalogs())
}

func (s *suite) m3CollectCatalogs(ctx context.Context, catalogs []m3Catalog) ([]row, map[string]rowEvidence) {
	var rows []row
	evidence := make(map[string]rowEvidence)
	for _, catalog := range catalogs {
		for _, name := range catalog.names {
			item := s.m3CollectedRow(ctx, catalog, name)
			rows = append(rows, item)
			evidence[item.category+"/"+item.name] = s.m3Evidence()
		}
	}
	return rows, evidence
}

// m3Evidence identifies every M3 row by the candidate it ran.
func (s *suite) m3Evidence() rowEvidence {
	return rowEvidence{Agent: binaryName, Version: m3CandidateVersion, Binary: s.digest}
}

// m3CollectedRow runs one case in the scope of its own home; a catalog
// pinned to another system than this one is skipped and not required,
// because the generated CI runs those cases where it runs.
func (s *suite) m3CollectedRow(ctx context.Context, catalog m3Catalog, name string) row {
	if catalog.only != "" && catalog.only != s.m2OS {
		return row{catalog.category, name, statusSkip, "required on " + catalog.only + ", the system the generated CI runs on", false}
	}
	return measuredM3(catalog.category, name, catalog.run(s.m3Scope(catalog.category+"-"+name), ctx, name))
}

// measuredM3 is the row measured builds, with the detail of a passing M3
// row.
func measuredM3(category, name string, err error) row {
	got := measured(category, name, err)
	if err == nil {
		got.detail = m3Verified
	}
	return got
}

// m3AcceptanceRows gives every command the M2 bound when it has none,
// prepares the candidate and collects the rows; a candidate that cannot be
// prepared is the one required row of the run.
func (s *suite) m3AcceptanceRows(ctx context.Context) ([]row, map[string]rowEvidence) {
	execute := s.execute
	s.execute = func(ctx context.Context, c command) result {
		if c.timeout == 0 {
			c.timeout = m2ProcessTimeout
		}
		return execute(ctx, c)
	}
	if err := s.prepareM3(ctx); err != nil {
		item := measuredM3(m3Preparation, candidateRow, err)
		return []row{item}, map[string]rowEvidence{item.category + "/" + item.name: s.m3Evidence()}
	}
	return s.collectM3(ctx)
}

func (s *suite) runM3(ctx context.Context, out io.Writer) int {
	rows, evidence := s.m3AcceptanceRows(ctx)
	return m2ReportCode(reportEvidence(out, rows, "", s.digest, evidence))
}
