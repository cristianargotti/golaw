package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/hook/codex"
)

// The M4 rows of section 11: the preparation row a failed build answers
// with, the detail of a passing row, and the evidence of a row that is no
// single agent's.
const (
	m4Preparation = "m4-preparation"
	m4Candidate   = "candidate"
	m4Verified    = "verified offline through the installed command"
	m4Unversioned = "none"
	m4AllAgents   = "all"
)

// m4Catalog is one category of M4 rows: the agent and the validated agent
// version its evidence names, and the rows it measures, given whether a
// skipped row is required.
type m4Catalog struct {
	agent, version string
	rows           func(s *suite, ctx context.Context, required bool) []row
}

// m4Catalogs are the categories of the M4 rows in report order.
func m4Catalogs() []m4Catalog {
	return []m4Catalog{
		{agent: agentCodex, version: codex.PatchGrammar, rows: (*suite).m4CodexRows},
		{agent: agentCursor, version: m4Unversioned, rows: (*suite).m4CursorRows},
		{agent: m4AllAgents, version: codex.PatchGrammar, rows: (*suite).m4ReasonRows},
		{agent: m4AllAgents, version: codex.PatchGrammar, rows: (*suite).m4FailureRows},
		{agent: m4AllAgents, version: codex.PatchGrammar, rows: (*suite).m4OrderRows},
		{agent: agentCodex, version: codex.PatchGrammar, rows: (*suite).m4TrustRows},
		{agent: agentCodex, version: codex.PatchGrammar, rows: (*suite).m4VendorRows},
		{agent: agentCodex, version: codex.PatchGrammar, rows: (*suite).m4LiveRows},
	}
}

// collectM4 measures every M4 category over a prepared suite.
func (s *suite) collectM4(ctx context.Context, required bool) ([]row, map[string]rowEvidence) {
	return s.m4CollectCatalogs(ctx, m4Catalogs(), required)
}

// m4CollectCatalogs walks the catalogs in order and keys the evidence of
// every row by category and name, as the M2 and M3 rows are keyed.
func (s *suite) m4CollectCatalogs(ctx context.Context, catalogs []m4Catalog, required bool) ([]row, map[string]rowEvidence) {
	var rows []row
	evidence := make(map[string]rowEvidence)
	for _, catalog := range catalogs {
		for _, item := range catalog.rows(s, ctx, required) {
			rows = append(rows, item)
			evidence[item.category+"/"+item.name] = rowEvidence{Agent: catalog.agent, Version: catalog.version, Binary: s.digest}
		}
	}
	return rows, evidence
}

// measuredM4 is measured with the detail of a passing M4 row.
func measuredM4(category, name string, err error) row {
	got := measured(category, name, err)
	if err == nil {
		got.detail = m4Verified
	}
	return got
}

// runM4 reports the M4 rows alone, over the preparation of the M1 rows:
// the real binary built and every agent installed twice in the isolated
// home. Each catalog names itself on stderr as it starts and once its rows
// are measured, because the report comes only at the end and a run its CI
// job cancels would otherwise leave no line saying where it stopped.
func (s *suite) runM4(ctx context.Context, out io.Writer) int {
	return s.m4Run(ctx, out, m4Announced(m4Catalogs(), os.Stderr))
}

// m4Announced wraps every catalog so that it writes one line to progress
// before its rows are measured and one after, with their number and their
// categories. The harness reads no clock: a CI log stamps each line.
func m4Announced(catalogs []m4Catalog, progress io.Writer) []m4Catalog {
	announced := make([]m4Catalog, 0, len(catalogs))
	for index, catalog := range catalogs {
		label := fmt.Sprintf("acceptance -m4: catalog %d of %d", index+1, len(catalogs))
		measure := catalog.rows
		catalog.rows = func(s *suite, ctx context.Context, required bool) []row {
			_, _ = fmt.Fprintln(progress, label, "started") //nolint:errcheck // A progress line never decides a row.
			rows := measure(s, ctx, required)
			var categories []string
			for _, item := range rows {
				if !slices.Contains(categories, item.category) {
					categories = append(categories, item.category)
				}
			}
			done := fmt.Sprintf("measured %s, rows: %d", strings.Join(categories, ", "), len(rows))
			_, _ = fmt.Fprintln(progress, label, done) //nolint:errcheck // A progress line never decides a row.
			return rows
		}
		announced = append(announced, catalog)
	}
	return announced
}

func (s *suite) m4Run(ctx context.Context, out io.Writer, catalogs []m4Catalog) int {
	if err := s.prepare(ctx); err != nil {
		item := measured(m4Preparation, m4Candidate, err)
		evidence := map[string]rowEvidence{m4Preparation + "/" + m4Candidate: {Agent: m4AllAgents, Binary: s.digest}}
		return m2ReportCode(reportEvidence(out, []row{item}, "", s.digest, evidence))
	}
	rows, evidence := s.m4CollectCatalogs(ctx, catalogs, false)
	return m2ReportCode(reportEvidence(out, rows, "", s.digest, evidence))
}

// m4Flags are the flags of the M4 runs: -m4 reports the M4 rows alone,
// -live-codex runs the live lane of D21 with the facts file it names.
type m4Flags struct {
	only *bool
	live *string
}

func m4FlagSet(flags *flag.FlagSet) m4Flags {
	return m4Flags{
		only: flags.Bool("m4", false, "require M4 functional evidence rows"),
		live: flags.String(m4LaneFlag, "", "record the Codex live session of macOS with the facts of this JSON file"),
	}
}

// route runs the M4 mode a flag selects and reports whether one did; a
// release run keeps every row, so -m4 never routes it.
func (f m4Flags) route(ctx context.Context, out io.Writer, s *suite, release bool) (int, bool) {
	switch {
	case *f.live != "":
		return s.runCodexLane(ctx, out, *f.live), true
	case *f.only && !release:
		return s.runM4(ctx, out), true
	}
	return 0, false
}
