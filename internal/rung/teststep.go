package rung

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

// testStep names the go test step test_staged arms, raceNeedsCgo is what go
// prints when -race meets a host whose cgo is off, and raceUnsupported what
// it prints on a platform the race detector does not cover.
const (
	testStep        = "test"
	raceNeedsCgo    = "-race requires cgo"
	raceUnsupported = "-race is not supported on"
)

// test runs go test -race over the affected packages when the ladder arms
// test_staged (decision 8: off by default, because the suite of a real
// commit took minutes; the full suite stays in make ci). The tests run with
// the user environment, so a test that spawns git answers for its own
// directory and never for the repository the hook runs in.
func (p *precommit) test(ctx context.Context, t goTree) Outcome {
	switch {
	case !p.ladder.TestStaged:
		return passed(testStep, "skipped, test_staged is off in the ladder of golaw.yml")
	case len(t.set.Patterns) == 0:
		return passed(testStep, noPackage)
	}
	out, err := runner.Run(ctx, p.goCommand(t, "test", "-race"))
	switch {
	case err != nil:
		return stepFailed(testStep, err)
	case out.Code == 0:
		return passed(testStep, fmt.Sprintf("go test -race passes in %d affected packages", len(t.set.Packages)))
	case bytes.Contains(out.Stderr, []byte(raceNeedsCgo)):
		return passed(testStep, "go test -race needs cgo and a C compiler this host lacks: install a C compiler or switch test_staged off in golaw.yml; the pipeline runs the tests")
	case bytes.Contains(out.Stderr, []byte(raceUnsupported)):
		return passed(testStep, "go test -race is not supported on this host: switch test_staged off in golaw.yml; the pipeline runs the tests")
	}
	return p.failures(ctx, out)
}

// failures judges a go test that failed: a failure in a package the commit
// touches refuses, unless each of its tests already fails at HEAD, and a
// failure anywhere else, or one no package names, is named and left to
// the pipeline.
func (p *precommit) failures(ctx context.Context, out runner.Output) Outcome {
	failed := failedPackages(out)
	touched := slices.DeleteFunc(slices.Clone(failed), func(pkg string) bool { return !p.touches(pkg) })
	var lines []string
	if len(touched) > 0 {
		held, ok := p.heldAtHead(ctx, out, touched)
		if !ok {
			return refusedBy(testStep, "go test -race fails in the affected packages; fix the tests before the commit", out)
		}
		lines = append(lines, stepLine(testStep, "go test -race fails at HEAD too in the packages the commit touches, which the pipeline judges: "+strings.Join(held, ", ")))
	}
	if others := slices.DeleteFunc(failed, p.touches); len(others) > 0 {
		lines = append(lines, stepLine(testStep, "go test -race fails in packages the commit does not touch, which the pipeline judges: "+strings.Join(others, ", ")))
	}
	if len(failed) == 0 {
		lines = append(lines, stepLine(testStep, "go test -race fails without naming a package, which the pipeline judges"))
	}
	return Outcome{Lines: slices.Concat(lines, toolLines(out))}
}

// failedPackages are the import paths go test names in a FAIL line, a
// package that did not build included, each once and in order.
func failedPackages(out runner.Output) []string {
	var failed []string
	for _, line := range toolLines(out) {
		rest, ok := strings.CutPrefix(line, failPrefix)
		if !ok {
			continue
		}
		pkg, _, _ := strings.Cut(rest, "\t")
		pkg, _, _ = strings.Cut(pkg, " ")
		if !slices.Contains(failed, pkg) {
			failed = append(failed, pkg)
		}
	}
	return failed
}

// touches reports whether the commit changes a file of the package of an
// import path, its test data included: a test that fails anywhere else, in
// an importer of a staged package, failed before the commit as far as the
// rung can know, because nothing of the commit lies in that package.
func (p *precommit) touches(pkg string) bool {
	module := p.modulePath()
	dir, ok := strings.CutPrefix(pkg, module+"/")
	if pkg == module {
		dir, ok = ".", true
	}
	return ok && module != "" && slices.ContainsFunc(changedPaths(p.scan.Input.Changes), func(path string) bool { return packageDir(path) == dir })
}

// modulePath is the module the go.mod of the index names, as the import
// paths go test prints start with it.
func (p *precommit) modulePath() string {
	data, err := p.scan.Index.Read(goMod)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], "\"`")
		}
	}
	return ""
}
