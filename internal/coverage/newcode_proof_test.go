package coverage_test

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/newcode"
)

const newCodeID = "coverage.new-code"

// newLinesFile stands for the diff in a proof fixture: one line per file,
// its slash path and its new line numbers, so the proof judges the line
// model and the verdict without a repository.
const newLinesFile = "newlines.txt"

// coverageNewProofCheck binds the new code evaluator to its fixtures.
type coverageNewProofCheck struct{}

func (coverageNewProofCheck) Spec() model.Spec {
	return model.Spec{
		ID: newCodeID, Rule: "rules/3@1",
		Scope: model.ScopeTree, Owner: model.OwnerCore,
		Proof: newCodeID,
	}
}

func (coverageNewProofCheck) Run(_ context.Context, in model.Input) (model.Result, error) {
	if !model.Any(in.Snapshot, func(path string) bool { return path == newLinesFile }) {
		return model.Result{Status: model.NotApplicable("no new lines in this fixture"), Scope: newCodeID}, nil
	}
	report, err := newCodeFixtureReport(in.Snapshot)
	if err != nil {
		return model.Result{}, err
	}
	result := model.Result{Status: model.Measured(), Scope: newCodeID}
	if !report.Passed() {
		result.Findings = append(result.Findings, model.Finding{
			Code: "coverage-new-code", Severity: model.SeverityFail,
			Message: strings.Join(report.Text(), "; "),
		})
	}
	return result, nil
}

func newCodeFixtureReport(s model.Snapshot) (newcode.Report, error) {
	files := make(map[string][]byte, 3)
	for _, name := range []string{"go.mod", "profile.cov", newLinesFile} {
		data, err := s.Read(name)
		if err != nil {
			return newcode.Report{}, fmt.Errorf("read %s: %w", name, err)
		}
		files[name] = data
	}
	module, err := newcode.Module(files["go.mod"])
	if err != nil {
		return newcode.Report{}, fmt.Errorf("fixture module: %w", err)
	}
	hits, err := newcode.Lines(bytes.NewReader(files["profile.cov"]), module, s.Read)
	if err != nil {
		return newcode.Report{}, fmt.Errorf("fixture report: %w", err)
	}
	added, err := parseNewLines(string(files[newLinesFile]))
	if err != nil {
		return newcode.Report{}, err
	}
	return newcode.Measure(newcode.Input{
		Lines: hits, NewLines: added,
		Universe:  func(path string) bool { return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") },
		Threshold: coverage.FloorsFrom(nil).NewCode,
	}), nil
}

func parseNewLines(text string) (map[string][]int, error) {
	out := make(map[string][]int)
	for line := range strings.SplitSeq(strings.TrimSpace(text), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("%w: newlines.txt line %q names no line", model.ErrUsage, line)
		}
		for _, field := range fields[1:] {
			number, err := strconv.Atoi(field)
			if err != nil {
				return nil, fmt.Errorf("newlines.txt line %q: %w", line, err)
			}
			out[fields[0]] = append(out[fields[0]], number)
		}
	}
	return out, nil
}
