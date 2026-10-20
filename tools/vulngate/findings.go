package main

import (
	"slices"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/vuln"
)

// verdictLines renders one line per module and fix of a reachable
// advisory, one per waived advisory and one per expired waiver, and
// reports whether anything is red: a reachable advisory without
// an unexpired waiver, or a waiver past its day. The judgment is the one
// the pre-push rung applies (internal/vuln); only the lane refuses an
// expired waiver of an advisory the code no longer reaches.
func verdictLines(found []vuln.Finding, waivers []model.Waiver, today time.Time) ([]string, bool) {
	verdict := vuln.Judge(found, waivers, today)
	var lines []string
	for i, f := range found {
		if w, ok := waiverOf(waivers, verdict, f); ok {
			if i == 0 || found[i-1].ID != f.ID {
				lines = append(lines, f.ID+" waived until "+w.Until+" ("+w.Record+")")
			}
			continue
		}
		lines = append(lines, f.ID+" reachable in "+f.Module+", fixed in "+fixedOr(f.Fixed))
	}
	expired := vuln.Expired(waivers, today)
	for _, w := range expired {
		lines = append(lines, "waiver "+w.ID+" expired on "+w.Until+", remove it")
	}
	if len(found) == 0 {
		lines = append([]string{"no reachable vulnerability"}, lines...)
	}
	return lines, len(verdict.Refused) > 0 || len(expired) > 0
}

// waiverOf answers the waiver in force of a waived advisory: loadWaivers
// keeps one entry per id, so the entry of its id is the one Judge held.
func waiverOf(waivers []model.Waiver, verdict vuln.Verdict, f vuln.Finding) (model.Waiver, bool) {
	if !slices.Contains(verdict.Waived, f) {
		return model.Waiver{}, false
	}
	i := slices.IndexFunc(waivers, func(w model.Waiver) bool { return w.ID == f.ID })
	return waivers[i], true
}

// fixedOr still renders an advisory without a fix, because that is the
// case a dated waiver exists for.
func fixedOr(version string) string {
	if version == "" {
		return "no version yet"
	}
	return version
}
