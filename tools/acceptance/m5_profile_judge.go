package main

import (
	"fmt"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
	scanreport "github.com/cristianargotti/golaw/internal/report"
)

// m5ToolTerms is the core check the extra terms of a profile reach (D6).
const m5ToolTerms = "text.tool-terms"

// m5Evaluators are the checks a profile turns on and the core check its
// terms reach (D5, D6), sorted: the synthetic fixture holds one violation
// of each.
func m5Evaluators() []string {
	return []string{
		"paths.profile-forbidden", "policy.image-installs", "policy.image-registry",
		"policy.image-stages", "policy.profile-targets", "policy.values-assertions", m5ToolTerms,
	}
}

// m5FindingChecks are the checks that carry a finding, each once, sorted.
func m5FindingChecks(doc scanreport.Document) []string {
	var checks []string
	for k := range doc.Findings {
		finding := &doc.Findings[k]
		if !slices.Contains(checks, finding.Check) {
			checks = append(checks, finding.Check)
		}
	}
	slices.Sort(checks)
	return checks
}

// m5JudgeProfiled holds the red tree under the profile: exit 1, nothing
// unmeasured, and every evaluator naming its violation with no other check
// finding anything, so the findings are the profile's alone.
func m5JudgeProfiled(doc scanreport.Document, code int) error {
	if code != 1 || doc.Counts.Unmeasured != 0 {
		return fmt.Errorf("the red tree under the profile exited %d with %d unmeasured, want 1 and none: %w", code, doc.Counts.Unmeasured, ErrAcceptance)
	}
	if got := m5FindingChecks(doc); !slices.Equal(got, m5Evaluators()) {
		return fmt.Errorf("checks with findings %q, want %q: %w", got, m5Evaluators(), ErrAcceptance)
	}
	return nil
}

// m5JudgeCore holds the same tree core only: exit 0 with no finding, and
// every profile check not applicable, so the profile alone made it red.
func m5JudgeCore(doc scanreport.Document, code int) error {
	if code != 0 || len(doc.Findings) != 0 {
		return fmt.Errorf("the tree core only exited %d with %d findings, want 0 and none: %w", code, len(doc.Findings), ErrAcceptance)
	}
	return m5ProfileStatuses(doc, model.StatusNotApplicable)
}

// m5JudgeCorrected holds the corrected tree under the profile: exit 0 with
// no finding, and every profile check measured, so the clean result is a
// measurement and never an inapplicability.
func m5JudgeCorrected(doc scanreport.Document, code int) error {
	if code != 0 || len(doc.Findings) != 0 {
		return fmt.Errorf("the corrected tree exited %d with %d findings, want 0 and none: %w", code, len(doc.Findings), ErrAcceptance)
	}
	return m5ProfileStatuses(doc, model.StatusMeasured)
}

// m5ProfileStatuses requires each of the six profile checks to answer kind.
func m5ProfileStatuses(doc scanreport.Document, kind model.StatusKind) error {
	for _, check := range m5Evaluators() {
		if check == m5ToolTerms {
			continue
		}
		if got := m3Status(doc, check).Status; got != kind.String() {
			return fmt.Errorf("%s answered %q, want %q: %w", check, got, kind.String(), ErrAcceptance)
		}
	}
	return nil
}
