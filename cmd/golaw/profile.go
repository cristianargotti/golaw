package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/runner"
)

// profileCommand is the human lifecycle of a profile bundle (A19): install,
// refresh, rollback and remove under the config directory, and the digest
// of a bundle directory for the person who authors one. None of them reads
// or writes a golaw.yml: the pin moves only through init --profile <name>
// --apply, committed by a person.
func (a *app) profileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Install, refresh, roll back or remove a profile bundle, or print the digest of one",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("%w: profile needs a subcommand (install, refresh, rollback, remove, digest)", model.ErrUsage)
		},
	}
	cmd.AddCommand(
		profileSubcommand("install <git url>@<tag>", "Clone a profile bundle at a tag, verify it and make it current", a.profileInstall),
		profileSubcommand("refresh <name>", "Install the current version of a profile again from where it came", a.profileRefresh),
		profileSubcommand("rollback <name>", "Make the previously installed version of a profile current again", a.profileRollback),
		profileSubcommand("remove <name>", "Remove every installed version of a profile and its record", a.profileRemove),
		profileSubcommand("digest <dir>", "Check rules.yml of a bundle directory and print the digest profile.yml carries", a.profileDigest),
	)
	return cmd
}

type profileRun func(ctx context.Context, out io.Writer, arg string) error

// profileSubcommand gives every subcommand one shape: one argument, and
// a failure named by the subcommand, so each exits 2 with one line.
func profileSubcommand(use, short string, run profileRun) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := run(cmd.Context(), cmd.OutOrStdout(), args[0]); err != nil {
				return fmt.Errorf("profile %s: %w", cmd.Name(), err)
			}
			return nil
		},
	}
}

// lifecycle answers the directories and the user environment every step
// runs with: git clones with the person's own configuration.
func (a *app) lifecycle() (policy.Dirs, []string, error) {
	b := a.bind()
	dirs, err := b.dirs()
	if err != nil {
		return policy.Dirs{}, nil, err
	}
	return dirs, runner.UserEnv(b.environ()), nil
}

func (a *app) profileInstall(ctx context.Context, out io.Writer, arg string) error {
	src, err := profile.ParseSource(arg)
	if err != nil {
		return err //nolint:wrapcheck // the usage error names the form; the subcommand wraps it
	}
	dirs, env, err := a.lifecycle()
	if err != nil {
		return err
	}
	installed, already, err := profile.Install(ctx, dirs, src, env, a.now())
	if err != nil {
		return err //nolint:wrapcheck // the lifecycle error names the bundle; the subcommand wraps it
	}
	return writeLines(out, installLines(installed, already))
}

func (a *app) profileRefresh(ctx context.Context, out io.Writer, name string) error {
	dirs, env, err := a.lifecycle()
	if err != nil {
		return err
	}
	installed, err := profile.Refresh(ctx, dirs, name, env, a.now())
	if err != nil {
		return err //nolint:wrapcheck // the lifecycle error names the bundle; the subcommand wraps it
	}
	return writeLines(out, refreshLines(installed))
}

func (a *app) profileRollback(_ context.Context, out io.Writer, name string) error {
	dirs, _, err := a.lifecycle()
	if err != nil {
		return err
	}
	installed, err := profile.Rollback(dirs, name, a.now())
	if err != nil {
		return err //nolint:wrapcheck // the lifecycle error names the bundle; the subcommand wraps it
	}
	return writeLines(out, rolledBackLines(installed))
}

func (a *app) profileRemove(_ context.Context, out io.Writer, name string) error {
	dirs, _, err := a.lifecycle()
	if err != nil {
		return err
	}
	if err := profile.Remove(dirs, name); err != nil {
		return err //nolint:wrapcheck // the lifecycle error names the bundle; the subcommand wraps it
	}
	return writeLines(out, []string{removedLine(name)})
}

// profileDigest checks rules.yml of a bundle being authored and prints the
// digest its profile.yml must carry; it reads no stamp, because the stamp
// is written from what this prints.
func (a *app) profileDigest(_ context.Context, out io.Writer, dir string) error {
	raw, err := os.ReadFile(filepath.Join(dir, profile.RulesFile)) //nolint:gosec // the person names the bundle directory
	if err != nil {
		return fmt.Errorf("read %s: %w", profile.RulesFile, err)
	}
	data, err := profile.Decode(raw)
	if err != nil {
		return err //nolint:wrapcheck // the bundle error names the field; the subcommand wraps it
	}
	if invalid := profile.Validate(data); invalid != nil {
		return invalid //nolint:wrapcheck // the bundle error names the field; the subcommand wraps it
	}
	digest, err := profile.Digest(dir)
	if err != nil {
		return err //nolint:wrapcheck // the bundle error names the entry; the subcommand wraps it
	}
	return writeLines(out, []string{digest})
}
