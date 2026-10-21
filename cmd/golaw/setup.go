package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

func (a *app) setupCommand() *cobra.Command {
	var uninstall bool
	cmd := &cobra.Command{
		Use:   "setup [path]",
		Short: "Arm optional Git hooks or remove the hooks recorded by setup",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// A person who stops setup stops it through the context, so a
			// stop after lefthook install still puts the git hooks back.
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			deps := setup.Dependencies{Run: runner.Run, Verify: runner.Verify}
			plan, err := a.bind().setupPlan(ctx, rootArg(args), a.now(), uninstall, deps)
			if err != nil {
				return err
			}
			return setupFailed(setup.Apply(ctx, plan, cmd.OutOrStdout(), deps))
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove the receipt-owned hooks and keep the tools installed")
	return cmd
}

func (b binding) setupPlan(ctx context.Context, path string, now time.Time, uninstall bool, deps setup.Dependencies) (setup.Plan, error) {
	optionsFor, planFor := b.setupOptions, setup.PlanInstall
	if uninstall {
		// Removing owned hooks must remain possible without Go or a valid policy.
		optionsFor, planFor = b.setupBaseOptions, setup.PlanUninstall
	}
	options, err := optionsFor(ctx, path, now)
	if err != nil {
		return setup.Plan{}, setupFailed(err)
	}
	plan, err := planFor(ctx, options, deps)
	return plan, setupFailed(err)
}

func setupFailed(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("setup: %w", err)
}
