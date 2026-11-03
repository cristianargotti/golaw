package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/genmap"
	"github.com/cristianargotti/golaw/internal/model"
)

// contextOptions are the flags of golaw context.
type contextOptions struct {
	budget int
	asJSON bool
}

// contextCommand prints the shapes of one area, so an agent that adds a
// feature reads the ports, use cases, types, adapters, handlers, tables
// and wiring in one turn instead of opening the neighboring files.
func (a *app) contextCommand() *cobra.Command {
	var opts contextOptions
	cmd := &cobra.Command{
		Use:   "context <area> [path]",
		Short: "Print the signatures of one area by layer: ports, use cases, types, adapters, handlers, tables, queries, wiring and the tests to copy",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.printContext(cmd.Context(), cmd.OutOrStdout(), args[0], rootArg(args[1:]), opts)
		},
	}
	cmd.Flags().IntVar(&opts.budget, "budget", genmap.DefaultContextBudget, "the tokens the text may take, estimated as bytes divided by four")
	cmd.Flags().BoolVar(&opts.asJSON, jsonFlag, false, "print the whole context as JSON")
	return cmd
}

// printContext refuses a repository outside golaw and an unknown area with
// exit 2, naming the areas there are.
func (a *app) printContext(ctx context.Context, out io.Writer, area, path string, opts contextOptions) error {
	if opts.budget < 1 {
		return fmt.Errorf("%w: context: --budget takes a positive number of tokens", model.ErrUsage)
	}
	root, err := a.discover(ctx, path)
	if err != nil {
		return contextFailed(err)
	}
	pol, err := a.load(ctx, root, a.now())
	if err != nil {
		return contextFailed(err)
	}
	if pol.Enrollment.State != model.EnrollmentEnrolled {
		return fmt.Errorf("%w: context: %s is not enrolled in golaw: run golaw init", model.ErrUsage, root)
	}
	c, areas, err := a.bind().areaContext(ctx, root, pol, area)
	if errors.Is(err, genmap.ErrNoArea) {
		return fmt.Errorf("%w: context: %s has no area %s: the areas are %s", model.ErrUsage, root, area, strings.Join(areas, ", "))
	}
	if err != nil {
		return contextFailed(err)
	}
	text := c.Text(opts.budget)
	if opts.asJSON {
		data, err := c.JSON()
		if err != nil {
			return contextFailed(err)
		}
		text = string(data) + "\n"
	}
	if _, err := io.WriteString(out, text); err != nil {
		return contextFailed(fmt.Errorf("write the context: %w", err))
	}
	return nil
}

func contextFailed(err error) error {
	return fmt.Errorf("context: %w", err)
}
