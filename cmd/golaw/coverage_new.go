package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/newcode"
)

const (
	coveragePlainFlag    = "plain-input"
	coverageNewCodeFlag  = "new-code"
	coverageBaseFlag     = "base"
	coverageSinceTagFlag = "since-tag"
)

// coverageRecordedFlags are the flags whose explicit use the command
// records, in the order a refusal names them, so --new-code can refuse
// the ones it would otherwise ignore.
func coverageRecordedFlags() []string {
	return []string{coverageTotalFlag, coveragePackageFlag, coverageFileFlag, coverageMinimumFlag, coveragePlainFlag}
}

// validateNew refuses what --new-code would silently ignore: two periods,
// a floor of the strict profile and a second profile.
func (o coverageOptions) validateNew() error {
	if o.Base != "" && o.SinceTag {
		return fmt.Errorf("%w: --base and --since-tag name two new code periods, give one", model.ErrUsage)
	}
	for _, name := range coverageRecordedFlags() {
		if o.Set[name] {
			return fmt.Errorf("%w: --%s does not apply to --new-code", model.ErrUsage, name)
		}
	}
	return nil
}

// runCoverageNew prints the measure of new code; an unmeasured report
// exits 2 with its reason, never 0 (M6.8 item 6).
func (a *app) runCoverageNew(ctx context.Context, out io.Writer, root string, options coverageOptions) error {
	report, header, err := a.bind().coverageNew(ctx, root, a.now(), options)
	if off, ok := errors.AsType[coverageOffError](err); ok {
		return writeLines(out, []string{"coverage --new-code: " + off.Error()})
	}
	if err != nil {
		return fmt.Errorf("coverage --new-code: %w", err)
	}
	if err := writeLines(out, append(header, report.Text()...)); err != nil {
		return fmt.Errorf("coverage --new-code output: %w", err)
	}
	switch {
	case report.Status.Kind == model.StatusUnmeasured:
		return fmt.Errorf("%w: %s", newcode.ErrUnmeasured, report.Status.Reason)
	case report.Status.Kind == model.StatusMeasured && !report.Passed():
		return exitError{code: exitFindings}
	}
	return nil
}
