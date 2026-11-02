package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/genmap"
	"github.com/cristianargotti/golaw/internal/model"
)

// mapOptions are the flags of golaw map.
type mapOptions struct {
	budget int
	area   string
	asJSON bool
}

// mapCommand prints the repository map of M7.8, so an agent reads the
// layers, the ports and the open holes once instead of exploring the tree.
func (a *app) mapCommand() *cobra.Command {
	var opts mapOptions
	cmd := &cobra.Command{
		Use:   "map [path]",
		Short: "Print the layers, ports, adapters, use cases, handlers, contracts and open holes of an enrolled repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.printMap(cmd.Context(), cmd.OutOrStdout(), rootArg(args), opts)
		},
	}
	cmd.Flags().IntVar(&opts.budget, "budget", genmap.DefaultBudget, "the tokens the text may take, estimated as bytes divided by four")
	cmd.Flags().StringVar(&opts.area, "area", "", "print one area only")
	cmd.Flags().BoolVar(&opts.asJSON, jsonFlag, false, "print the whole map as JSON")
	return cmd
}

// printMap refuses a repository outside golaw with exit 2, because the map
// reads the generate block of an enrolled policy and nothing else.
func (a *app) printMap(ctx context.Context, out io.Writer, path string, opts mapOptions) error {
	if opts.budget < 1 {
		return fmt.Errorf("%w: map: --budget takes a positive number of tokens", model.ErrUsage)
	}
	root, err := a.discover(ctx, path)
	if err != nil {
		return mapFailed(err)
	}
	pol, err := a.load(ctx, root, a.now())
	if err != nil {
		return mapFailed(err)
	}
	if pol.Enrollment.State != model.EnrollmentEnrolled {
		return fmt.Errorf("%w: map: %s is not enrolled in golaw: run golaw init", model.ErrUsage, root)
	}
	report, err := a.bind().repositoryMap(ctx, root, pol, opts.area)
	if err != nil {
		return mapFailed(err)
	}
	text := report.Text(opts.budget)
	if opts.asJSON {
		data, err := report.JSON()
		if err != nil {
			return mapFailed(err)
		}
		text = string(data) + "\n"
	}
	if _, err := io.WriteString(out, text); err != nil {
		return mapFailed(fmt.Errorf("write output: %w", err))
	}
	return nil
}

func mapFailed(err error) error {
	return fmt.Errorf("map: %w", err)
}
