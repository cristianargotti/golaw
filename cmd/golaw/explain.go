package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/model"
)

// configCommand groups the policy inspection commands; explain prints every
// value of the policy in force with its source (specification section 5.3).
// A bare config is a usage error, so it never exits 0 having done nothing.
func (a *app) configCommand() *cobra.Command {
	config := &cobra.Command{
		Use:   "config",
		Short: "Inspect the policy of a repository",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("%w: config needs a subcommand (explain)", model.ErrUsage)
		},
	}
	explain := &cobra.Command{
		Use:   "explain [path]",
		Short: "Print every value of the policy in force and where it comes from",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			text, err := a.explain(cmd.Context(), rootArg(args), a.now())
			if err != nil {
				return fmt.Errorf("config explain: %w", err)
			}
			if _, err := io.WriteString(cmd.OutOrStdout(), text); err != nil {
				return fmt.Errorf("config explain: write output: %w", err)
			}
			return nil
		},
	}
	config.AddCommand(explain)
	return config
}
