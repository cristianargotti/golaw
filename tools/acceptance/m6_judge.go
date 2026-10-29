package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// m6Key identifies the findings one expectation counts.
type m6Key struct{ path, check, code string }

// m6Document decodes the one document scan --json or lint --json prints.
func m6Document(stdout string) (scanreport.Document, error) {
	var doc scanreport.Document
	decoder := json.NewDecoder(strings.NewReader(stdout))
	if err := decoder.Decode(&doc); err != nil {
		return scanreport.Document{}, fmt.Errorf("decode the report: %w", err)
	}
	if decoder.More() {
		return scanreport.Document{}, fmt.Errorf("the report holds a second document: %w", ErrAcceptance)
	}
	return doc, nil
}

// m6Tally counts findings by key, notes included.
func m6Tally(findings []model.Finding) map[m6Key]int {
	out := make(map[m6Key]int)
	for i := range findings {
		out[m6Key{findings[i].Path, findings[i].Check, findings[i].Code}]++
	}
	return out
}

// m6Want sums the counts of the expectations of one source: the lint tools
// when lint, the native checks otherwise.
func m6Want(exps []surfacefix.Expectation, lint bool) map[m6Key]int {
	out := make(map[m6Key]int)
	for _, e := range exps {
		if strings.HasPrefix(e.Check, m6LintPrefix) == lint {
			out[m6Key{e.Path, e.Check, e.Code}] += e.Count
		}
	}
	return out
}

// m6JudgeRed requires the red twin's findings of one source to hold every
// class count exactly. A finding no class claims is allowed: the strict set
// raises more on the red twin than the classes count, and row (1) asks for
// the class counts only.
func m6JudgeRed(got result, exps []surfacefix.Expectation, lint bool) error {
	if got.err != nil {
		return fmt.Errorf("run the candidate: %w", got.err)
	}
	if got.code != 1 {
		return fmt.Errorf("exit %d, want 1 for findings: %w", got.code, ErrAcceptance)
	}
	doc, err := m6Document(got.stdout)
	if err != nil {
		return err
	}
	tally, want := m6Tally(doc.Findings), m6Want(exps, lint)
	for _, key := range slices.SortedFunc(maps.Keys(want), m6Compare) {
		if tally[key] != want[key] {
			return fmt.Errorf("%s %s %s: %d findings, want %d: %w", key.path, key.check, key.code, tally[key], want[key], ErrAcceptance)
		}
	}
	return nil
}

// m6Compare orders keys so a failure always names the same key first.
func m6Compare(a, b m6Key) int {
	return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.check, b.check), cmp.Compare(a.code, b.code))
}

// m6JudgeClean requires a report with no finding, notes included, and no
// check left unmeasured; the exit code is judged last so the refusal names
// the finding or the check that caused it.
func m6JudgeClean(got result) error {
	if got.err != nil {
		return fmt.Errorf("run the candidate: %w", got.err)
	}
	doc, err := m6Document(got.stdout)
	if err != nil {
		return err
	}
	if len(doc.Findings) > 0 {
		f := doc.Findings[0]
		return fmt.Errorf("%s:%d %s %s: want zero findings: %w", f.Path, f.Line, f.Check, f.Code, ErrAcceptance)
	}
	for _, check := range doc.Checks {
		if check.Status == m6Unmeasured {
			return fmt.Errorf("%s unmeasured: %s: %w", check.ID, check.Reason, ErrAcceptance)
		}
	}
	if doc.Counts.Unmeasured > 0 {
		return fmt.Errorf("%d checks unmeasured: %w", doc.Counts.Unmeasured, ErrAcceptance)
	}
	if got.code != 0 {
		return fmt.Errorf("exit %d, want 0 for a clean report: %w", got.code, ErrAcceptance)
	}
	return nil
}

// m6JudgeTwin judges a red twin by its class counts, a corrected one clean.
func m6JudgeTwin(got result, twin surfacefix.Twin, lint bool) error {
	if twin == surfacefix.Red {
		return m6JudgeRed(got, surfacefix.Expectations(), lint)
	}
	return m6JudgeClean(got)
}

// m6Unmeasured is the status of a required check that could not run.
const m6Unmeasured = "unmeasured"
