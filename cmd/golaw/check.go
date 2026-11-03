package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/check"
)

// checkTimeout bounds a whole golaw check run; the pipeline's test target
// allows ten minutes, and the gates run at once inside the same budget.
const checkTimeout = 10 * time.Minute

// checkOptions is one golaw check invocation.
type checkOptions struct {
	base            string
	baseNamed       bool // --base was given, so a base git cannot resolve is a usage error
	asJSON, verbose bool
	timeout         time.Duration
}

// checkCommand runs every gate make ci fails on over the change of the
// worktree and prints only what is red, so an agent verifies its work in
// one turn instead of one turn per tool.
func (a *app) checkCommand() *cobra.Command {
	var opts checkOptions
	cmd := &cobra.Command{
		Use:   "check [path]",
		Short: "Run the gates the pipeline fails on over the change of the worktree and print only what is red",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.baseNamed = cmd.Flags().Changed(coverageBaseFlag)
			return a.check(cmd.Context(), cmd.OutOrStdout(), rootArg(args), opts)
		},
	}
	cmd.Flags().StringVar(&opts.base, coverageBaseFlag, "", "the ref the change is judged against; the remote default branch when empty")
	cmd.Flags().BoolVar(&opts.asJSON, jsonFlag, false, "print the gates as JSON instead of the text lines")
	cmd.Flags().BoolVar(&opts.verbose, "verbose", false, "also name the green gates")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", checkTimeout, "the budget of the whole run")
	return cmd
}

// check plans the gates, runs them at once under the budget and prints the
// report: exit 0 when every gate is green or skipped, 1 when one is red.
func (a *app) check(ctx context.Context, out io.Writer, root string, opts checkOptions) error {
	ctx, cancel := context.WithTimeout(ctx, opts.timeout)
	defer cancel()
	plan, err := a.checkPlan(ctx, root, opts)
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}
	defer plan.close()
	report := plan.report(check.Run(ctx, plan.gates()))
	if err := writeCheck(out, report, opts); err != nil {
		return fmt.Errorf("check: %w", err)
	}
	if report.Red() {
		return exitError{code: exitFindings}
	}
	return nil
}

func writeCheck(out io.Writer, report check.Report, opts checkOptions) error {
	if opts.asJSON {
		return writeJSON(out, report)
	}
	return writeLines(out, report.Lines(opts.verbose))
}
