package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// scanCommand runs the rules over the worktree snapshot of a repository
// and maps the decision to the exit code; --json selects the JSON report.
func (a *app) scanCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Run the rules over the worktree of a repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.scan(cmd.Context(), cmd.OutOrStdout(), rootArg(args), asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the JSON report instead of the text report")
	return cmd
}

// rootArg is the repository path of a command: the argument, or the
// current directory when none is given.
func rootArg(args []string) string {
	if len(args) == 0 {
		return "."
	}
	return args[0]
}

// scan loads the policy in force, freezes the worktree snapshot, dispatches
// the catalog over every file of it and renders the decision. Findings and
// an unmeasured required check leave as an exitError after the report.
func (a *app) scan(ctx context.Context, out io.Writer, root string, asJSON bool) error {
	policy, err := a.load(ctx, root, a.now())
	if err != nil {
		return scanFailed(err)
	}
	snapshot, err := a.snapshot(ctx, root)
	if err != nil {
		return scanFailed(err)
	}
	registry, err := a.registry()
	if err != nil {
		return scanFailed(err)
	}
	in := model.Input{Mode: model.ModeScan, Policy: policy, Snapshot: snapshot, Targets: snapshot.Files()}
	run, err := engine.Dispatch(ctx, registry, in)
	if err != nil {
		return scanFailed(err)
	}
	decision := engine.Decide(run)
	if err := a.render(out, in, run, decision, asJSON); err != nil {
		return scanFailed(err)
	}
	if code := codeOf(decision); code != exitClean {
		return exitError{code: code}
	}
	return nil
}

// scanFailed names the command in front of a seam or engine failure.
func scanFailed(err error) error {
	return fmt.Errorf("scan: %w", err)
}

// codeOf maps a decision to the exit code of section 4.4: a required check
// left unmeasured is a failure before any finding is a finding.
func codeOf(d engine.Decision) int {
	switch {
	case len(d.Unmeasured) > 0:
		return exitFailure
	case d.Verdict == engine.VerdictClean:
		return exitClean
	default:
		return exitFindings
	}
}
