package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/hook/coexist"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

// The installation scopes of specification section 5.6.
const (
	scopeUser    = "user"
	scopeProject = "project"
)

// agentOptions are the flags the agent commands share.
type agentOptions struct {
	agent string
	scope string
}

// agentCommand groups the installers: install writes the artifacts of an
// agent after printing the diff and enrolls the repository; uninstall
// removes exactly what the receipt lists (specification section 5.6).
func (a *app) agentCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "agent", Short: "Install or remove the kit's artifacts of an agent"}
	cmd.AddCommand(a.agentInstallCommand(), a.agentUninstallCommand())
	return cmd
}

// agentFlags adds the flags the two subcommands share.
func agentFlags(cmd *cobra.Command, opts *agentOptions) {
	cmd.Flags().StringVar(&opts.agent, "agent", "", "the agent: "+strings.Join(agents.IDs(), ", "))
	cmd.Flags().StringVar(&opts.scope, "scope", scopeUser, "where the artifacts go: "+scopeUser+" or "+scopeProject)
}

// agentRun is the body of one agent subcommand.
type agentRun func(ctx context.Context, out io.Writer, path string, opts agentOptions) error

func (a *app) agentInstallCommand() *cobra.Command {
	return agentSubcommand("install [path]",
		"Install the skill and the hooks of an agent after printing the diff, and enroll the repository",
		a.agentInstall)
}

func (a *app) agentUninstallCommand() *cobra.Command {
	return agentSubcommand("uninstall [path]", "Remove exactly what the receipt of an agent lists", a.agentUninstall)
}

// agentSubcommand builds one subcommand over the shared flags and the
// optional path argument.
func agentSubcommand(use, short string, run agentRun) *cobra.Command {
	var opts agentOptions
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), cmd.OutOrStdout(), rootArg(args), opts)
		},
	}
	agentFlags(cmd, &opts)
	return cmd
}

// agentInstall refuses to arm a second blocking verdict (A17), plans the
// installation, prints the diff, enrolls the repository through the port
// and writes the artifacts.
func (a *app) agentInstall(ctx context.Context, out io.Writer, path string, opts agentOptions) error {
	agent, root, err := a.agentTarget(ctx, path, opts)
	if err != nil {
		return installFailed(err)
	}
	if refusal := a.refuseOwnedFamilies(out, root); refusal != nil {
		return refusal
	}
	options, err := a.installOptions(ctx, agent, root, opts.scope, a.now())
	if err != nil {
		return installFailed(err)
	}
	plan, err := agent.Plan(options)
	if err != nil {
		return installFailed(err)
	}
	port, err := a.enrollments(root, a.now())
	if err != nil {
		return installFailed(err)
	}
	if err := install.Apply(ctx, plan, out, port); err != nil {
		return installFailed(err)
	}
	return installFailed(writeLines(out, []string{"installed: " + plan.ReceiptPath}))
}

// agentUninstall removes the artifacts of the receipt and the receipt.
func (a *app) agentUninstall(ctx context.Context, out io.Writer, path string, opts agentOptions) error {
	agent, root, err := a.agentTarget(ctx, path, opts)
	if err != nil {
		return uninstallFailed(err)
	}
	receipt, err := a.receiptPath(agent, root, opts.scope)
	if err != nil {
		return uninstallFailed(err)
	}
	plan, err := install.PlanUninstall(receipt)
	if err != nil {
		return uninstallFailed(err)
	}
	if err := install.Apply(ctx, plan, out, nil); err != nil {
		return uninstallFailed(err)
	}
	return uninstallFailed(writeLines(out, []string{"uninstalled: " + receipt}))
}

// agentTarget validates the flags and resolves the repository root.
func (a *app) agentTarget(ctx context.Context, path string, opts agentOptions) (agents.Agent, string, error) {
	if opts.agent == "" {
		return agents.Agent{}, "", fmt.Errorf("%w: --agent is required, known: %s", model.ErrUsage, strings.Join(agents.IDs(), ", "))
	}
	agent, err := lookupAgent(opts.agent)
	if err != nil {
		return agents.Agent{}, "", err
	}
	if opts.scope != scopeUser && opts.scope != scopeProject {
		return agents.Agent{}, "", fmt.Errorf("%w: --scope must be %s or %s", model.ErrUsage, scopeUser, scopeProject)
	}
	root, err := a.discover(ctx, path)
	if err != nil {
		return agents.Agent{}, "", err
	}
	return agent, root, nil
}

// refuseOwnedFamilies prints the ownership table and refuses, with exit 1,
// while the other gate still owns a family the kit would arm (A17).
func (a *app) refuseOwnedFamilies(out io.Writer, root string) error {
	home, err := a.home()
	if err != nil {
		return installFailed(err)
	}
	detection, err := coexist.Detect(home, root)
	if err != nil {
		return installFailed(err)
	}
	table := coexist.Build(detection)
	refusal := table.Refuse()
	if refusal == nil {
		return nil
	}
	if werr := writeLines(out, append(table.Lines(), "refused: "+refusal.Error())); werr != nil {
		return installFailed(werr)
	}
	return exitError{code: exitFindings}
}

// installFailed and uninstallFailed name the command in front of a failure
// and pass nil through.
func installFailed(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("agent install: %w", err)
}

func uninstallFailed(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("agent uninstall: %w", err)
}
