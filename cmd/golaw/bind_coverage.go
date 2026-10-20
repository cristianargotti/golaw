package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/policy"
)

// coverage measures the root of the repository selected by the positional path,
// while profile paths remain relative to the invocation directory.
func (b binding) coverage(
	ctx context.Context, root string, now time.Time,
	options coverageOptions,
) (coverage.Report, *coverage.Report, error) {
	if err := options.validate(); err != nil {
		return coverage.Report{}, nil, err
	}
	loaded, err := b.loaded(ctx, root, now)
	if err != nil {
		return coverage.Report{}, nil, err
	}
	if checkoutErr := enrolledCheckout(loaded); checkoutErr != nil {
		return coverage.Report{}, nil, checkoutErr
	}
	floors := coverage.FloorsFrom(loaded.Policy.OptionsFor(policy.CoverageCheck))
	floors = tightenedCoverage(floors, options)
	local, err := evaluateCoverageFile(options.Input, loaded.Root, floors)
	if err != nil {
		return coverage.Report{}, nil, err
	}
	if options.PlainInput == "" {
		return local, nil, nil
	}
	plain, err := evaluateCoverageFile(options.PlainInput, loaded.Root, floors)
	if err != nil {
		return coverage.Report{}, nil, err
	}
	return local, &plain, nil
}

// tightenedCoverage only applies stricter flags (section 5.3), so a Makefile
// cannot lower a floor raised by the policy in force.
func tightenedCoverage(floors coverage.Floors, options coverageOptions) coverage.Floors {
	if options.Set[coverageTotalFlag] {
		floors.Total = max(floors.Total, options.Floors.Total)
	}
	if options.Set[coveragePackageFlag] {
		floors.Package = max(floors.Package, options.Floors.Package)
	}
	if options.Set[coverageFileFlag] {
		floors.File = max(floors.File, options.Floors.File)
	}
	if options.Set[coverageMinimumFlag] {
		floors.MinStatements = min(floors.MinStatements, options.Floors.MinStatements)
	}
	return floors
}

func evaluateCoverageFile(name, root string, floors coverage.Floors) (coverage.Report, error) {
	profile, err := os.Open(name) //nolint:gosec // the caller explicitly selects the coverage profile to read
	if err != nil {
		return coverage.Report{}, fmt.Errorf("open coverage profile %q: %w", name, err)
	}
	report, err := readCoverageProfile(profile, root, floors)
	if err != nil {
		return coverage.Report{}, fmt.Errorf("evaluate coverage profile %q: %w", name, err)
	}
	return report, nil
}

func readCoverageProfile(profile io.ReadCloser, root string, floors coverage.Floors) (coverage.Report, error) {
	report, readErr := coverage.Evaluate(profile, root, floors)
	closeErr := profile.Close()
	return report, errors.Join(readErr, closeErr)
}
