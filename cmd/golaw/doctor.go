package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/model"
)

// doctorCommand reports, per agent, what is installed, enrolled and
// verified for a repository (specification section 5.6): the artifacts by
// hash, the binary the hook resolves, the plugin, the read deny rules, the
// fail mode, the versions, the build metadata, the enrollment and the
// coexistence with the other gate. It is a report: the exit code is 0 once
// it is printed and 2 only when it cannot be collected.
func (a *app) doctorCommand() *cobra.Command {
	var asJSON bool
	var scope string
	cmd := &cobra.Command{
		Use:   "doctor [path]",
		Short: "Report what is installed, enrolled and verified for every agent",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.doctor(cmd.Context(), cmd.OutOrStdout(), rootArg(args), scope, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the JSON report, one document per agent")
	cmd.Flags().StringVar(&scope, "scope", scopeUser, "the installation scope to inspect: "+scopeUser+" or "+scopeProject)
	return cmd
}

// doctor collects and prints one report per agent. Outside git the real
// path of the argument stands for the repository, so the installed
// artifacts are still reported.
func (a *app) doctor(ctx context.Context, out io.Writer, path, scope string, asJSON bool) error {
	if scope != scopeUser && scope != scopeProject {
		return doctorFailed(fmt.Errorf("%w: --scope must be %s or %s", model.ErrUsage, scopeUser, scopeProject))
	}
	root, err := a.discover(ctx, path)
	if err != nil {
		if root, err = filepath.Abs(path); err != nil {
			return doctorFailed(fmt.Errorf("resolve %s: %w", path, err))
		}
	}
	for _, agent := range agents.All() {
		report, err := a.doctorReport(ctx, agent, root, scope, a.now())
		if err != nil {
			return doctorFailed(err)
		}
		if !asJSON {
			if err := writeLines(out, []string{"agent: " + agent.ID}); err != nil {
				return doctorFailed(err)
			}
		}
		if err := doctor.Print(out, report, asJSON); err != nil {
			return doctorFailed(fmt.Errorf("print the report: %w", err))
		}
	}
	return nil
}

// doctorFailed names the command in front of a failure.
func doctorFailed(err error) error {
	return fmt.Errorf("doctor: %w", err)
}
