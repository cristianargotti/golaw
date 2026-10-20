package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// scanCommand runs the rules over the worktree snapshot of a repository,
// or over its index with --staged, and maps the decision to the exit code;
// --json selects the JSON report.
func (a *app) scanCommand() *cobra.Command {
	var asJSON, staged bool
	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Run the rules over the worktree of a repository, or over its index with --staged",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if staged {
				in := policy.Input{Dir: rootArg(args), Now: a.now()}
				return a.scanStaged(cmd.Context(), cmd.OutOrStdout(), in, asJSON)
			}
			return a.scan(cmd.Context(), cmd.OutOrStdout(), rootArg(args), asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the JSON report instead of the text report")
	cmd.Flags().BoolVar(&staged, "staged", false, "judge the index, the next commit, and report only the findings it introduces")
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
// the catalog over every file of it and renders the decision.
func (a *app) scan(ctx context.Context, out io.Writer, root string, asJSON bool) error {
	inForce, err := a.load(ctx, root, a.now())
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
	in := model.Input{Mode: model.ModeScan, Policy: inForce, Snapshot: snapshot, Targets: snapshot.Files()}
	run, err := engine.Dispatch(ctx, registry, in)
	if err != nil {
		return scanFailed(err)
	}
	return a.decided(out, in, run, asJSON)
}

// scanStaged judges the index of the repository that holds in.Dir: the
// policy in force, the proposed policy judged apart and only the findings
// the next commit introduces, rendered and mapped to the exit code as scan.
func (a *app) scanStaged(ctx context.Context, out io.Writer, in policy.Input, asJSON bool) error {
	registry, err := a.registry()
	if err != nil {
		return scanFailed(err)
	}
	result, err := a.bind().staged(ctx, in, registry)
	if err != nil {
		return scanFailed(err)
	}
	return a.decided(out, result.Input, result.Run, asJSON)
}

// decided renders a run and maps its decision to the exit code: findings
// and a required check left unmeasured leave as an exitError after the
// report.
func (a *app) decided(out io.Writer, in model.Input, run *engine.Run, asJSON bool) error {
	decision := engine.Decide(run)
	if err := a.render(out, in, run, decision, asJSON); err != nil {
		return scanFailed(err)
	}
	if code := codeOf(decision); code != exitClean {
		return exitError{code: code}
	}
	return nil
}

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
