package newcode

import (
	"cmp"
	"maps"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

// Input is what one measure of new code reads.
type Input struct {
	// Lines are the lines to cover of the pipeline report, per slash path.
	Lines map[string]LineHits
	// NewLines are the line numbers the change adds, per slash path.
	NewLines map[string][]int
	// Universe admits the paths the analyzer scope measures; nil admits all.
	Universe func(path string) bool
	// Threshold is the policy's floor; zero means the Sonar way value.
	Threshold int
}

// File is the measure of one file the change touches.
type File struct {
	// Path is the slash path of the file.
	Path string
	// ToCover and Covered count the new lines to cover and those covered.
	ToCover, Covered int
	// Uncovered lists the new lines to cover no test ran, ascending.
	Uncovered []int
}

// Report is the measure of a change against the threshold.
type Report struct {
	// ToCover, Covered and Threshold are the totals and the floor applied.
	ToCover, Covered, Threshold int
	// Files holds every file with a new line to cover, sorted by path.
	Files []File
	// Status is Measured, NotApplicable without a new line to cover, or
	// Unmeasured when the report names no file of the universe.
	Status model.Status
}

// Measure counts the new lines to cover of the universe the way the gate
// does: only lines the report holds count, and no line to cover is never 100.
func Measure(in Input) Report {
	report := Report{Threshold: cmp.Or(in.Threshold, model.DefaultCoverageNew)}
	admits := in.Universe
	if admits == nil {
		admits = func(string) bool { return true }
	}
	if !slices.ContainsFunc(slices.Collect(maps.Keys(in.Lines)), admits) {
		report.Status = model.Unmeasured("the report names no file of the coverage universe")
		return report
	}
	for _, path := range slices.Sorted(maps.Keys(in.NewLines)) {
		hits, named := in.Lines[path]
		if !named || !admits(path) {
			continue
		}
		file := measureFile(path, hits, in.NewLines[path])
		if file.ToCover == 0 {
			continue
		}
		report.Files = append(report.Files, file)
		report.ToCover += file.ToCover
		report.Covered += file.Covered
	}
	report.Status = model.Measured()
	if report.ToCover == 0 {
		report.Status = model.NotApplicable("no new line to cover")
	}
	return report
}

func measureFile(path string, hits LineHits, added []int) File {
	lines := slices.Clone(added)
	slices.Sort(lines)
	file := File{Path: path}
	for _, line := range slices.Compact(lines) {
		covered, toCover := hits[line]
		switch {
		case !toCover:
		case covered:
			file.ToCover++
			file.Covered++
		default:
			file.ToCover++
			file.Uncovered = append(file.Uncovered, line)
		}
	}
	return file
}
