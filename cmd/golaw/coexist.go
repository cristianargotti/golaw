package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/hook/coexist"
)

// coexistCommand prints the ownership table of A17 for a repository: which
// gate owns the blocking verdict on each hook event and rule family. The
// exit code is 1 while the other gate still owns a family the kit would
// arm, so an installer that consults it never arms a second verdict.
func (a *app) coexistCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "coexist [path]",
		Short: "Print which gate owns each hook event and rule family on a repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := a.home()
			if err != nil {
				return coexistFailed(err)
			}
			detection, err := coexist.Detect(home, rootArg(args))
			if err != nil {
				return coexistFailed(err)
			}
			table := coexist.Build(detection)
			if err := writeLines(cmd.OutOrStdout(), table.Lines()); err != nil {
				return coexistFailed(err)
			}
			refusal := table.Refuse()
			if refusal == nil {
				return nil
			}
			if err := writeLines(cmd.OutOrStdout(), []string{"refused: " + refusal.Error()}); err != nil {
				return coexistFailed(err)
			}
			return exitError{code: exitFindings}
		},
	}
}

func coexistFailed(err error) error {
	return fmt.Errorf("coexist: %w", err)
}
