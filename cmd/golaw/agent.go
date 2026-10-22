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

type agentOptions struct {
	agent string
	scope string
	every bool // the command runs under --agent all
}

// agentCommand groups the installers: install writes the artifacts of an
// agent after printing the diff and enrolls the repository; uninstall
// removes exactly what the receipt lists (specification section 5.6).
func (a *app) agentCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "agent", Short: "Install or remove the kit's artifacts of an agent"}
	cmd.AddCommand(a.agentInstallCommand(), a.agentUninstallCommand())
	return cmd
}

func agentFlags(cmd *cobra.Command, opts *agentOptions) {
	cmd.Flags().StringVar(&opts.agent, "agent", "", "the agent: "+strings.Join(agents.IDs(), ", ")+", or "+agentAll+" for every one")
	cmd.Flags().StringVar(&opts.scope, "scope", scopeUser, "where the artifacts go: "+scopeUser+" or "+scopeProject)
}

type agentRun func(ctx context.Context, out io.Writer, path string, opts agentOptions) error

func (a *app) agentInstallCommand() *cobra.Command {
	return agentSubcommand("install [path]",
		"Install the skill and the hooks of an agent after printing the diff, and enroll the repository",
		a.everyAgent(a.agentInstall))
}

func (a *app) agentUninstallCommand() *cobra.Command {
	return agentSubcommand("uninstall [path]", "Remove exactly what the receipt of an agent lists", a.everyAgent(a.uninstallInstalled))
}

// agentSubcommand gives install and uninstall one shape, so their flags and
// their path argument cannot drift apart.
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
	if refusal := a.refuseOwnedFamilies(out, agent, root); refusal != nil {
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
	if err := reportEnrollment(ctx, out, port, plan); err != nil {
		return installFailed(err)
	}
	if err := install.Apply(ctx, plan, out, port); err != nil {
		return installFailed(err)
	}
	return installFailed(writeLines(out, installedLines(plan)))
}

// reportEnrollment names a re-enrollment before the diff (A3): the record
// holds another policy, and agent install is about to enroll the policy in
// force, which loaded as a tightening of it.
func reportEnrollment(ctx context.Context, out io.Writer, port enrollmentPort, plan install.Plan) error {
	current, err := port.ReadEnrollment(ctx, plan.Receipt.ProjectID, plan.PolicyDigest, plan.ProfileDigest)
	if err != nil {
		return fmt.Errorf("read the enrollment: %w", err)
	}
	if current.State != model.EnrollmentMismatch {
		return nil
	}
	return writeLines(out, []string{"enrollment: policy " + current.PolicyDigest + " becomes " + plan.PolicyDigest})
}

// agentUninstall goes by the receipt alone, so it removes only what the kit
// wrote and leaves the person's own files and settings entries.
func (a *app) agentUninstall(ctx context.Context, out io.Writer, path string, opts agentOptions) error {
	agent, root, err := a.agentTarget(ctx, path, opts)
	if err != nil {
		return uninstallFailed(err)
	}
	receipt, err := a.receiptPath(agent, root, opts.scope)
	if err != nil {
		return uninstallFailed(err)
	}
	plan, err := agent.Uninstall(receipt)
	if err != nil {
		return uninstallFailed(err)
	}
	if err := install.Apply(ctx, plan, out, nil); err != nil {
		return uninstallFailed(err)
	}
	return uninstallFailed(writeLines(out, []string{"uninstalled: " + receipt}))
}

// agentTarget checks the flags before it discovers the repository, so a
// usage error never depends on the directory the command runs in.
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
// while the other gate still owns a family the kit would arm (A17). The
// gate is read from the settings the host reads, the configuration
// directory of the agent, the one agent install writes.
func (a *app) refuseOwnedFamilies(out io.Writer, agent agents.Agent, root string) error {
	home, err := a.home()
	if err != nil {
		return installFailed(err)
	}
	detection, err := coexist.Detect(home, agent.ConfigDir(a.env, home), root)
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

// installFailed and uninstallFailed pass nil through, so a command returns
// its last write through them and every exit reads the same way.
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
