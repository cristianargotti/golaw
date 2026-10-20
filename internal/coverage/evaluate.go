package coverage

import (
	"fmt"
	"io"
	"maps"
	"path"
	"slices"
	"strings"
)

// Evaluate joins profile blocks to active sources so omitted measurements cannot answer clean.
func Evaluate(profile io.Reader, root string, floors Floors) (Report, error) {
	files, err := parseProfile(profile)
	if err != nil {
		return Report{}, err
	}
	module, err := modulePath(root)
	if err != nil {
		return Report{}, err
	}
	sources, err := sourceFiles(root, module)
	if err != nil {
		return Report{}, err
	}
	return evaluate(files, sources, module, floors)
}

func evaluate(files map[string]*fileCoverage, sources []string, module string, floors Floors) (Report, error) {
	report := Report{Total: Measure{Name: "total"}}
	packages := make(map[string]Measure)
	missing := make(map[string]bool)
	slices.Sort(sources)
	for _, source := range sources {
		name := strings.TrimPrefix(source, module+"/")
		file := Measure{Name: name}
		fc, ok := files[source]
		if !ok {
			missing[name] = true
		} else {
			count, err := fc.totals()
			if err != nil {
				return Report{}, fmt.Errorf("coverage file %q: %w", name, err)
			}
			file.Statements, file.Covered = count.stmts, count.covered
			if err := aggregate(&report.Total, packages, path.Dir(source), file); err != nil {
				return Report{}, err
			}
		}
		report.Files = append(report.Files, file)
	}
	for _, name := range slices.Sorted(maps.Keys(packages)) {
		report.Packages = append(report.Packages, packages[name])
	}
	report.judge(floors, missing)
	return report, nil
}

func aggregate(total *Measure, packages map[string]Measure, name string, file Measure) error {
	const maxStatements = int(^uint(0) >> 1)
	if file.Statements > maxStatements-total.Statements {
		return fmt.Errorf("%w: total statement count overflows at %q", ErrProfile, file.Name)
	}
	pkg := packages[name]
	pkg.Name = name
	pkg.Statements += file.Statements
	pkg.Covered += file.Covered
	packages[name] = pkg
	total.Statements += file.Statements
	total.Covered += file.Covered
	return nil
}

func (r *Report) judge(floors Floors, missing map[string]bool) {
	if r.Total.Percent() < floors.Total {
		r.Failures = append(r.Failures, r.Total)
	}
	for _, pkg := range r.Packages {
		r.judgeUnit(pkg, floors.Package, floors.MinStatements, false)
	}
	for _, file := range r.Files {
		if missing[file.Name] {
			r.Failures = append(r.Failures, file)
			continue
		}
		r.judgeUnit(file, floors.File, floors.MinStatements, slices.Contains(floors.Exemptions, file.Name))
	}
}

func (r *Report) judgeUnit(unit Measure, floor float64, minimum int, exempt bool) {
	if unit.Statements < minimum {
		r.Skipped = append(r.Skipped, unit)
		return
	}
	if !exempt && unit.Percent() < floor {
		r.Failures = append(r.Failures, unit)
	}
}
