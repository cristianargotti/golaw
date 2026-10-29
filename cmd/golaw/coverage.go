package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/coverage"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// coverageOptions keeps explicit flag state in Set so defaults never count as overrides.
type coverageOptions struct {
	Input      string
	PlainInput string
	Base       string
	NewCode    bool
	SinceTag   bool
	Floors     coverage.Floors
	Set        map[string]bool
}

const (
	coverageTotalFlag   = "total"
	coveragePackageFlag = "package"
	coverageFileFlag    = "file"
	coverageMinimumFlag = "min-statements"
	coveragePercentCap  = 100
	goCapability        = "go"
)

// coverageOffError is the answer of the coverage floors while the go
// capability is off: the policy names no Go program, so no profile is
// read and nothing is judged.
type coverageOffError struct{}

func (coverageOffError) Error() string { return "not applicable, the go capability is off" }

func (a *app) coverageCommand() *cobra.Command {
	options := coverageOptions{Floors: coverage.DefaultFloors()}
	cmd := &cobra.Command{
		Use:   "coverage [path]",
		Short: "Evaluate coverage profiles against the policy floors",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Set = make(map[string]bool)
			for _, name := range coverageRecordedFlags() {
				options.Set[name] = cmd.Flags().Changed(name)
			}
			switch {
			case options.NewCode:
				return a.runCoverageNew(cmd.Context(), cmd.OutOrStdout(), rootArg(args), options)
			case options.Base != "" || options.SinceTag:
				return fmt.Errorf("%w: --base and --since-tag need --new-code", model.ErrUsage)
			}
			return a.runCoverage(cmd.Context(), cmd.OutOrStdout(), rootArg(args), options)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&options.Input, "input", "", "coverage profile to evaluate (required); with --new-code, default the report the analyzer scope names")
	flags.StringVar(&options.PlainInput, coveragePlainFlag, "", "second coverage profile to evaluate independently")
	flags.Float64Var(&options.Floors.Total, coverageTotalFlag, options.Floors.Total, "tighten the total coverage floor")
	flags.Float64Var(&options.Floors.Package, coveragePackageFlag, options.Floors.Package, "tighten the package coverage floor")
	flags.Float64Var(&options.Floors.File, coverageFileFlag, options.Floors.File, "tighten the file coverage floor")
	flags.IntVar(&options.Floors.MinStatements, coverageMinimumFlag, options.Floors.MinStatements, "lower the statement minimum")
	flags.BoolVar(&options.NewCode, coverageNewCodeFlag, false, "measure coverage on new code the way the pipeline's gate does")
	flags.StringVar(&options.Base, coverageBaseFlag, "", "the base of a pull request; default the merge base with the remote default branch")
	flags.BoolVar(&options.SinceTag, coverageSinceTagFlag, false, "measure since the last tag reachable from HEAD")
	return cmd
}

func (a *app) runCoverage(ctx context.Context, out io.Writer, root string, options coverageOptions) error {
	local, plain, err := a.bind().coverage(ctx, root, a.now(), options)
	if off, ok := errors.AsType[coverageOffError](err); ok {
		return writeLines(out, []string{"coverage: " + off.Error()})
	}
	if err != nil {
		return fmt.Errorf("coverage: %w", err)
	}
	if err := writeLines(out, coverageLines(local, plain)); err != nil {
		return fmt.Errorf("coverage output: %w", err)
	}
	if len(local.Failures) > 0 || plain != nil && len(plain.Failures) > 0 {
		return exitError{code: exitFindings}
	}
	return nil
}

// validate uses the policy schema's bounds so CLI flags accept the same limits.
func (o coverageOptions) validate() error {
	if o.Input == "" {
		return fmt.Errorf("%w: --input is required", model.ErrUsage)
	}
	for _, flag := range []struct {
		name  string
		value float64
	}{
		{coverageTotalFlag, o.Floors.Total},
		{coveragePackageFlag, o.Floors.Package},
		{coverageFileFlag, o.Floors.File},
	} {
		if o.Set[flag.name] && (math.IsNaN(flag.value) || math.IsInf(flag.value, 0) ||
			flag.value < policy.DefaultCoverage || flag.value > coveragePercentCap) {
			return fmt.Errorf("%w: --%s must be a finite percentage from %d through %d",
				model.ErrUsage, flag.name, policy.DefaultCoverage, coveragePercentCap)
		}
	}
	if o.Set[coverageMinimumFlag] && (o.Floors.MinStatements < 0 || o.Floors.MinStatements > policy.DefaultMinStatements) {
		return fmt.Errorf("%w: --min-statements must be from 0 through %d", model.ErrUsage, policy.DefaultMinStatements)
	}
	return nil
}
