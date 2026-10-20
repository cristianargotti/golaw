package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/agents"
	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

// The exit codes of the command tree (specification section 4.4): clean,
// findings, and usage or failure or a required check left unmeasured.
const (
	exitClean    = 0
	exitFindings = 1
	exitFailure  = 2
)

// enrollmentPort is the enrollment record of A3 as the installer writes it
// and doctor reads it; the policy package owns the record, cmd binds it.
type enrollmentPort interface {
	install.EnrollmentWriter
	doctor.EnrollmentReader
}

// app holds the outputs and the seams the commands run through, so a test
// replaces each seam with a fake and reads the exit code without os.Exit.
// Every seam is bound once, in newApp.
type app struct {
	stdout, stderr io.Writer
	stdin          io.Reader
	buildInfo      func() buildinfo.Info
	now            func() time.Time
	home           func() (string, error)
	env            func(string) string // the environment the commands read, such as an agent's configuration override
	registry       func() (*engine.Registry, error)
	snapshot       func(ctx context.Context, root string) (model.Snapshot, error)
	load           func(ctx context.Context, root string, now time.Time) (*model.EffectivePolicy, error)
	compile        func(data []byte, now time.Time) (*model.EffectivePolicy, error)
	enroll         func(root string, data []byte, policy *model.EffectivePolicy, now time.Time) error
	committed      func(ctx context.Context, root, digest string) (bool, error)
	explain        func(ctx context.Context, root string, now time.Time) (string, error)
	render         func(w io.Writer, in model.Input, run *engine.Run, d engine.Decision, asJSON bool) error
	discover       func(ctx context.Context, dir string) (string, error)
	hookDeps       func(agent agents.Agent, now time.Time) (hook.Dependencies, error)
	installOptions func(ctx context.Context, agent agents.Agent, root, scope string, now time.Time) (install.Options, error)
	receiptPath    func(agent agents.Agent, root, scope string) (string, error)
	enrollments    func(root string, now time.Time) (enrollmentPort, error)
	doctorReport   func(ctx context.Context, agent agents.Agent, root, scope string, now time.Time) (doctor.Report, error)
	bind           func() binding // the production binding the rungs, setup and the ladder helpers reach without a seam of their own
}

// exitError carries an exit code that is a result, not a failure: findings
// are reported by the command itself, so run prints nothing for it.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

// run executes the command tree over the arguments and returns the exit
// code; every failure is printed once, prefixed with the binary name.
func (a *app) run(ctx context.Context, args []string) int {
	root := a.rootCommand() //nolint:contextcheck // cobra hands ctx to every RunE through ExecuteContext
	root.SetArgs(args)
	root.SetOut(a.stdout)
	root.SetErr(a.stderr)
	return a.exitCode(root.ExecuteContext(ctx))
}

// rootCommand builds the tree. Cobra's own error and usage printing are
// silenced so that every message leaves through exitCode in one shape.
func (a *app) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "golaw",
		Short:         "Go engineering standards: scan a tree, guard the agents, set a repository up",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(a.versionCommand(), a.scanCommand(), a.configCommand(), a.initCommand(), a.coexistCommand(), a.hookCommand(), a.agentCommand(), a.doctorCommand(), a.setupCommand(), a.coverageCommand(), a.affectedCommand(), a.pushRangeCommand(), a.lockCommand())
	return root
}

// exitCode maps a command error to the process exit code: nil is clean, an
// exitError is its code, anything else is a failure printed on stderr.
func (a *app) exitCode(err error) int {
	var exit exitError
	switch {
	case err == nil:
		return exitClean
	case errors.As(err, &exit):
		return exit.code
	default:
		a.diagnose(err.Error())
		return exitFailure
	}
}

// diagnose prints one failure line; a diagnostic write that fails has no
// better channel than the exit code, so its error is dropped on purpose.
func (a *app) diagnose(message string) {
	_, _ = io.WriteString(a.stderr, "golaw: "+message+"\n") //nolint:errcheck // the exit code is the remaining channel
}

// writeLines prints lines to a command's stdout and reports the write error
// to the caller, which wraps it with the command name.
func writeLines(w io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}
	return nil
}
