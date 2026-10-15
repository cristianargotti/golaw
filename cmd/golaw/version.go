package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCommand prints the stamped build metadata as metadata: the last
// line always says the executable is a source build and not verifiable.
func (a *app) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version and the build metadata of this binary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := writeLines(cmd.OutOrStdout(), a.buildInfo().Lines()); err != nil {
				return fmt.Errorf("version: %w", err)
			}
			return nil
		},
	}
}
