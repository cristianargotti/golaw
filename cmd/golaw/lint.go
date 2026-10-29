package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/engine"
)

// lintOptions is one golaw lint invocation.
type lintOptions struct {
	surface, asJSON bool
	set             string
	only            []string
}

func (a *app) lintCommand() *cobra.Command {
	var opts lintOptions
	cmd := &cobra.Command{
		Use:   "lint [path]",
		Short: "Run the pinned surface linters over the analyzer scope with the kit configurations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.lint(cmd.Context(), cmd.OutOrStdout(), rootArg(args), opts)
		},
	}
	// --surface is accepted so the lint-surface target spells what it runs;
	// the surface is all golaw lint reads.
	cmd.Flags().BoolVar(&opts.surface, surfaceFlag, false, "lint the non-Go surface; the Go linters stay in make lint")
	cmd.Flags().StringVar(&opts.set, "set", "", "parity or strict; the policy's surface.rules when empty")
	cmd.Flags().StringSliceVar(&opts.only, "only", nil, "a tool or a surface language; repeat for several")
	cmd.Flags().BoolVar(&opts.asJSON, jsonFlag, false, "print the JSON report instead of the text report")
	return cmd
}

// lint runs the surface linters and maps the decision to the exit code: a
// required tool missing or a file left Unmeasured exits 2 (M6.9).
func (a *app) lint(ctx context.Context, out io.Writer, path string, opts lintOptions) error {
	b := a.bind()
	in, run, err := b.lint(ctx, path, a.now(), opts)
	if err != nil {
		return fmt.Errorf("lint: %w", err)
	}
	decision := engine.Decide(run)
	if err := b.render(out, in, run, decision, opts.asJSON); err != nil {
		return fmt.Errorf("lint: %w", err)
	}
	if code := codeOf(decision); code != exitClean {
		return exitError{code: code}
	}
	return nil
}
