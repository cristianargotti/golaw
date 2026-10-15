package main

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
)

// floors are the percentage every scope must reach and the statement count
// under which a package or a file is too small to be held to it.
type floors struct {
	percent    float64
	statements int
}

// report is the evaluation result: one line per scope and the entries
// under the floor.
type report struct {
	Lines    []string
	Failures []string
}

// String renders the lines followed by the failures.
func (r *report) String() string {
	out := make([]string, 0, len(r.Lines)+len(r.Failures))
	out = append(out, r.Lines...)
	for _, f := range r.Failures {
		out = append(out, "FAIL "+f)
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// evaluate computes the total, the package and the file percentages.
// A source absent from the profile counts as one uncovered statement, so
// it fails the file floor and lowers its package.
func evaluate(files map[string]*fileCoverage, sources []string, f floors) report {
	perFile := make(map[string]tally, len(files))
	for name, fc := range files {
		perFile[name] = fc.totals()
	}
	for _, src := range sources {
		if _, ok := perFile[src]; !ok {
			perFile[src] = tally{stmts: 1}
		}
	}
	perPackage := make(map[string]tally)
	var total tally
	for name, t := range perFile {
		pkg := path.Dir(name)
		agg := perPackage[pkg]
		agg.stmts += t.stmts
		agg.covered += t.covered
		perPackage[pkg] = agg
		total.stmts += t.stmts
		total.covered += t.covered
	}
	var rep report
	rep.add("total", map[string]tally{"all": total}, floors{percent: f.percent})
	rep.add("package", perPackage, f)
	rep.add("file", perFile, f)
	return rep
}

// add appends the sorted lines of one scope and records its failures; a
// scope below the statement minimum is printed and never fails.
func (r *report) add(scope string, tallies map[string]tally, f floors) {
	for _, name := range slices.Sorted(maps.Keys(tallies)) {
		t := tallies[name]
		r.Lines = append(r.Lines, fmt.Sprintf("%-7s %6.1f%% %5d/%-5d %s", scope, t.percent(), t.covered, t.stmts, name))
		if t.stmts < f.statements {
			continue
		}
		if t.percent() < f.percent {
			r.Failures = append(r.Failures, fmt.Sprintf("%s %s %.1f%%", scope, name, t.percent()))
		}
	}
}
