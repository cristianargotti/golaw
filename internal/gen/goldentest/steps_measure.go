package goldentest

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/dup"
	"github.com/cristianargotti/golaw/internal/genheader"
	"github.com/cristianargotti/golaw/internal/gittest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/newcode"
	"github.com/cristianargotti/golaw/internal/source"
	"github.com/cristianargotti/golaw/internal/surface"
)

// The gate values of M7.10.1: the new code coverage floor and the ceiling
// of duplicated new lines.
const (
	coverageFloor   = 80
	duplicationCeil = 1.5
)

// openProfile is the cover profile with the holes open, and adopterProfile
// the one the adopter's make test writes, -coverpkg over every package.
const (
	openProfile    = "open.cov"
	adopterProfile = "adopter.cov"
)

// newLines answers the lines the slice adds since its base commit.
func (h *harness) newLines(ctx context.Context) (map[string][]int, error) {
	lines, err := newcode.NewLines(newcode.WithEnviron(ctx, gittest.Isolated()), h.repo.Root, h.base)
	if err != nil {
		return nil, fmt.Errorf("new lines since the base: %w", err)
	}
	return lines, nil
}

// lines reads a cover profile of the module into the lines it credits.
func (h *harness) lines(profile string) (map[string]newcode.LineHits, error) {
	file, err := os.Open(profile) //nolint:gosec // the profile is the harness's own
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filepath.Base(profile), err)
	}
	defer func() { _ = file.Close() }()
	read := func(path string) ([]byte, error) { return os.ReadFile(source.OSPath(h.repo.Root, path)) }
	hits, err := newcode.Lines(file, h.s.Module, read)
	if err != nil {
		return nil, fmt.Errorf(readFailed, filepath.Base(profile), err)
	}
	return hits, nil
}

// openCoverage proves the generated suites alone cover every regenerable
// line while the holes are open; the red tests of the holes fail by
// design, so the exit code is ignored.
func (h *harness) openCoverage(ctx context.Context) {
	profile := filepath.Join(h.work, openProfile)
	if _, err := goCommand(ctx, h.repo.Root, pipelineTest(profile)...); err != nil {
		h.failf(string(StepCoverage), "%v", err)
		return
	}
	if _, err := os.Stat(profile); err != nil {
		h.failf(string(StepCoverage), "no profile with the holes open")
		return
	}
	hits, err := h.lines(profile)
	if err != nil {
		h.failf(string(StepCoverage), "%v", err)
		return
	}
	before := len(h.failures)
	for _, path := range h.regenerable() {
		h.judgeOpen(path, hits)
	}
	h.report.OpenCovered = len(h.failures) == before
}

// regenerable answers the written regenerable Go files that are not tests.
func (h *harness) regenerable() []string {
	var out []string
	for _, path := range slices.Sorted(maps.Keys(h.report.Files)) {
		if !strings.HasSuffix(path, goSuffix) || strings.HasSuffix(path, testSuffix) {
			continue
		}
		if _, ok := genheader.Parse(h.report.Files[path]); ok {
			out = append(out, path)
		}
	}
	return out
}

// judgeOpen fails each line of one regenerable file no block ran.
func (h *harness) judgeOpen(path string, hits map[string]newcode.LineHits) {
	file, measured := hits[path]
	if !measured {
		if hasStatements(path, h.report.Files[path]) {
			h.failf(string(StepCoverage), "%s: no test of its package covers this regenerable file", path)
		}
		return
	}
	for _, line := range slices.Sorted(maps.Keys(file)) {
		if !file[line] {
			h.failf(string(StepCoverage), "%s:%d regenerable line not covered with the holes open", path, line)
		}
	}
}

// hasStatements reports a file with a function whose body holds a
// statement; a file of declarations alone has nothing to cover.
func hasStatements(path string, content []byte) bool {
	file, err := parser.ParseFile(token.NewFileSet(), path, content, parser.SkipObjectResolution)
	if err != nil {
		return true
	}
	return slices.ContainsFunc(file.Decls, func(d ast.Decl) bool {
		fn, ok := d.(*ast.FuncDecl)
		return ok && fn.Body != nil && len(fn.Body.List) > 0
	})
}

// coverage measures the new code of the filled slice the way the gate
// does, over the pipeline profile.
func (h *harness) coverage(ctx context.Context) {
	if h.profile == "" {
		profile := filepath.Join(h.work, pipelineProfile)
		if !h.goStep(ctx, StepCoverage, pipelineTest(profile)...) {
			return
		}
		h.profile = profile
	}
	in, err := h.measureInput(ctx)
	if err != nil {
		h.failf(string(StepCoverage), "%v", err)
		return
	}
	measured := newcode.Measure(in)
	h.report.Coverage = measured.Percent()
	if measured.Status.Kind == model.StatusUnmeasured {
		h.failf(string(StepCoverage), "unmeasured: %s", measured.Status.Reason)
		return
	}
	if !measured.Passed() {
		h.failf(string(StepCoverage), "new code %.1f percent, at least %d: %s", measured.Percent(), coverageFloor, uncovered(measured))
	}
	h.floors(ctx)
}

// floors holds the filled slice to the floors the adopter's coverage-check
// applies, total, per package and per file, over the profile its make test
// writes, through the code golaw coverage runs.
func (h *harness) floors(ctx context.Context) {
	profile := filepath.Join(h.work, adopterProfile)
	if !h.goStep(ctx, StepCoverage, goTest, goCount, "-coverpkg=./...", "-covermode=atomic", "-coverprofile="+profile, everyPackage) {
		return
	}
	data, err := os.ReadFile(profile) //nolint:gosec // the profile the step above wrote into the harness's own directory
	if err != nil {
		h.failf(string(StepCoverage), "read the adopter profile: %v", err)
		return
	}
	report, err := coverage.Evaluate(bytes.NewReader(data), h.repo.Root, coverage.DefaultFloors())
	if err != nil {
		h.failf(string(StepCoverage), "the adopter's floors: %v", err)
		return
	}
	for _, m := range report.Failures {
		h.failf(string(StepCoverage), "below the floor of the adopter's coverage-check: %s %.1f percent (%d/%d)", m.Name, m.Percent(), m.Covered, m.Statements)
	}
}

// measureInput gathers what newcode.Measure reads for the filled slice.
func (h *harness) measureInput(ctx context.Context) (newcode.Input, error) {
	hits, err := h.lines(h.profile)
	if err != nil {
		return newcode.Input{}, err
	}
	lines, err := h.newLines(ctx)
	if err != nil {
		return newcode.Input{}, err
	}
	snap, err := snapshot(ctx, h.repo.Root)
	if err != nil {
		return newcode.Input{}, err
	}
	scope, err := surface.ScopeOf(ctx, snap, h.policy)
	if err != nil {
		return newcode.Input{}, fmt.Errorf("analyzer scope: %w", err)
	}
	return newcode.Input{Lines: hits, NewLines: lines, Universe: newcode.Universe(snap, scope), Threshold: coverageFloor}, nil
}

// uncovered names the uncovered new lines of each file.
func uncovered(r newcode.Report) string {
	var parts []string
	for _, f := range r.Files {
		if len(f.Uncovered) != 0 {
			parts = append(parts, fmt.Sprintf("%s %v", f.Path, f.Uncovered))
		}
	}
	return strings.Join(parts, "; ")
}

// duplication measures the duplicated new lines of the slice over every
// Go file of the module outside testdata.
func (h *harness) duplication(ctx context.Context) {
	files, err := h.goSources(ctx)
	if err != nil {
		h.failf(string(StepDup), "%v", err)
		return
	}
	lines, err := h.newLines(ctx)
	if err != nil {
		h.failf(string(StepDup), "%v", err)
		return
	}
	measured, err := dup.Measure(files, lines, dup.Default())
	if err != nil {
		h.failf(string(StepDup), "%v", err)
		return
	}
	h.report.Duplication = measured.Percent()
	if h.report.Duplication >= duplicationCeil {
		h.failf(string(StepDup), "%.2f percent of new lines duplicated, under %.1f: %s", h.report.Duplication, duplicationCeil, blocks(measured))
	}
}

// goSources reads every Go file of the module outside testdata.
func (h *harness) goSources(ctx context.Context) (map[string][]byte, error) {
	snap, err := snapshot(ctx, h.repo.Root)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, f := range snap.Files() {
		if !strings.HasSuffix(f.Path, goSuffix) || slices.Contains(strings.Split(f.Path, "/"), "testdata") {
			continue
		}
		content, err := snap.Read(f.Path)
		if err != nil {
			return nil, fmt.Errorf(readFailed, f.Path, err)
		}
		files[f.Path] = content
	}
	return files, nil
}

// blocks names the spans of each duplicated block.
func blocks(r dup.Report) string {
	parts := make([]string, 0, len(r.Blocks))
	for _, b := range r.Blocks {
		spans := make([]string, 0, len(b.Spans))
		for _, s := range b.Spans {
			spans = append(spans, fmt.Sprintf("%s:%d-%d", s.Path, s.Start, s.End))
		}
		parts = append(parts, strings.Join(spans, " = "))
	}
	return strings.Join(parts, "; ")
}
