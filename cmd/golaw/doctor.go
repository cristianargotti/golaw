package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/model"
)

// The release line and the state that makes doctor --release exit 1.
const (
	doctorRelease = "doctor.release"
	stateRed      = "red"
)

// doctorCommand reports, per agent, what is installed, enrolled and
// verified for a repository (specification section 5.6): the artifacts by
// hash, the binary the hook resolves, the plugin, the read deny rules, the
// fail mode, the versions, the build metadata, the enrollment and the
// coexistence with the other gate, the pinned profile bundle and the
// release of the running binary. It is a report: the exit code is 0 once
// it is printed and 2 only when it cannot be collected, except that a red
// release line under --release exits 1, because a person runs that flag to
// decide whether to trust the binary (D10).
func (a *app) doctorCommand() *cobra.Command {
	var opts doctorOptions
	cmd := &cobra.Command{
		Use:   "doctor [path]",
		Short: "Report what is installed, enrolled and verified for every agent",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.doctor(cmd.Context(), cmd.OutOrStdout(), rootArg(args), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.asJSON, jsonFlag, false, "print the JSON report, one document per agent")
	cmd.Flags().StringVar(&opts.scope, "scope", scopeUser, "the installation scope to inspect: "+scopeUser+" or "+scopeProject)
	cmd.Flags().StringVar(&opts.release, "release", "", "verify the running binary against checksums.txt, its bundle and the attestation downloaded into this directory")
	cmd.Flags().StringVar(&opts.trustRoot, "trust-root", "", "verify against the roots.crt and rekor.pub of this directory instead of the embedded trust root")
	return cmd
}

// doctorOptions are the flags of doctor.
type doctorOptions struct {
	scope, release, trustRoot string
	asJSON                    bool
}

// doctor collects and prints one report per agent. Outside git the real
// path of the argument stands for the repository, so the installed
// artifacts are still reported, and the lines of a repository are left
// out.
func (a *app) doctor(ctx context.Context, out io.Writer, path string, opts doctorOptions) error {
	if opts.scope != scopeUser && opts.scope != scopeProject {
		return doctorFailed(fmt.Errorf("%w: --scope must be %s or %s", model.ErrUsage, scopeUser, scopeProject))
	}
	root, err := a.discover(ctx, path)
	repository := err == nil
	if err != nil {
		if root, err = filepath.Abs(path); err != nil {
			return doctorFailed(fmt.Errorf("resolve %s: %w", path, err))
		}
	}
	reports, err := a.doctorReports(ctx, root, opts, repository)
	if err != nil {
		return doctorFailed(err)
	}
	if err := printDoctorReports(out, reports, opts.asJSON); err != nil {
		return doctorFailed(err)
	}
	if opts.release != "" && redRelease(reports) {
		return exitError{code: exitFindings}
	}
	return nil
}

// redRelease reports whether the release line of the reports is red.
func redRelease(reports []doctor.Report) bool {
	for _, report := range reports {
		for _, line := range report.Lines {
			if line.ID == doctorRelease && line.State == stateRed {
				return true
			}
		}
	}
	return false
}

func doctorFailed(err error) error {
	return fmt.Errorf("doctor: %w", err)
}
