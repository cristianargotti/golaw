package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// newOptions are the flags of golaw new; each kind reads only its own.
type newOptions struct {
	port, usecase, api, in, out string
	ports, usecases             []string
	dryRun, asJSON              bool
}

// addendumArgs is the argument count of the addendum's adapter spelling,
// adapter sql <name>, beside the two of every other kind.
const addendumArgs = 3

// newExamples is one line per kind, so an agent copies the spelling and
// the flags instead of learning them from refusals.
const newExamples = `  golaw new port growth.Ledger --usecase ReadPlan
  golaw new usecase growth.ReadPlan --ports Ledger,Clock --in AccountID --out Plan
  golaw new adapter sql postgres.Plans --port growth.Ledger
  golaw new adapter http metagraph.Client --port meta.Graph
  golaw new handler cockpitapi --usecases growth`

// newCommand writes one kind instance with its holes (M7.8), so an agent
// starts from the shape and the tests and fills the domain logic only.
func (a *app) newCommand() *cobra.Command {
	var opts newOptions
	cmd := &cobra.Command{
		Use:     "new <kind> <name>",
		Short:   "Write a port, adapter (sql or http), use case or handler with its holes, red tests and manifest entry",
		Example: newExamples,
		Args:    cobra.RangeArgs(2, addendumArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.newKind(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), args, opts); err != nil {
				return fmt.Errorf("new: %w", err)
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.port, "port", "", "adapters: the port the adapter implements, as <area>.<Type>")
	flags.StringVar(&opts.usecase, "usecase", "", "port: the use case that consumes the port")
	flags.StringSliceVar(&opts.ports, "ports", nil, "usecase: the ports the use case consumes")
	flags.StringVar(&opts.in, "in", "", "usecase: the input type")
	flags.StringVar(&opts.out, "out", "", "usecase: the output type")
	flags.StringVar(&opts.api, "api", "", "handler: the OpenAPI document, the generate block's one when empty")
	flags.StringSliceVar(&opts.usecases, "usecases", nil, "handler: the use case areas the handler calls")
	flags.BoolVar(&opts.dryRun, "dry-run", false, "print the complete diff and write nothing")
	flags.BoolVar(&opts.asJSON, jsonFlag, false, "print the files written and the holes open as JSON")
	return cmd
}

// newRequest reads the arguments and the flags into the kind and its
// request; three arguments are the addendum's spelling adapter sql <name>.
func newRequest(args []string, opts newOptions) (string, genkind.Request, error) {
	kind := args[0]
	if len(args) == addendumArgs {
		if args[0] != "adapter" || (args[1] != "sql" && args[1] != "http") {
			return "", genkind.Request{}, fmt.Errorf("%w: three arguments spell adapter sql or adapter http, then the name", genkind.ErrRequest)
		}
		kind = "adapter-" + args[1]
	}
	name, err := parseName(args[len(args)-1])
	if err != nil {
		return "", genkind.Request{}, err
	}
	req := genkind.Request{
		Name: name, Usecase: opts.usecase, Ports: opts.ports,
		In: opts.in, Out: opts.out, API: opts.api, Usecases: opts.usecases,
	}
	if opts.port != "" {
		if req.Port, err = parseName(opts.port); err != nil {
			return "", genkind.Request{}, err
		}
	}
	return kind, req, nil
}

// parseName reads <area>.<Type> or a bare package name.
func parseName(s string) (genkind.Name, error) {
	area, typ, dotted := strings.Cut(s, ".")
	if area == "" || (dotted && (typ == "" || strings.Contains(typ, "."))) || strings.ContainsFunc(s, unicode.IsSpace) {
		return genkind.Name{}, fmt.Errorf("%w: %q is not <area>.<Type> or a package name", genkind.ErrRequest, s)
	}
	return genkind.Name{Area: area, Type: typ}, nil
}

// newKind plans the instance, then prints the diff of a dry run or lands
// the writes and reports them. A skipped write is exit 1, because the run
// did not write everything it planned.
func (a *app) newKind(ctx context.Context, out, errOut io.Writer, args []string, opts newOptions) error {
	kind, req, err := newRequest(args, opts)
	if err != nil {
		return err
	}
	run, err := a.prepareGen(ctx, ".")
	if err != nil {
		return err
	}
	title := newTitle{kind: kind, name: args[len(args)-1], asJSON: opts.asJSON}
	res, err := gen.New(ctx, run.input, kind, req)
	if errors.Is(err, genkind.ErrExists) {
		return refuseNew(out, title, err)
	}
	if err != nil {
		return fmt.Errorf("plan %s %s: %w", kind, title.name, err)
	}
	if opts.dryRun {
		return writeLines(out, dryRunLines(kind, title.name, res.Writes, run.snapshot, len(res.Open)))
	}
	report, err := apply.Files(run.root, res.Writes)
	if err != nil {
		// What landed before the failure is printed, so the person knows
		// which files the next run finds.
		return errors.Join(fmt.Errorf("write %s %s: %w", kind, title.name, err), writeLines(out, newLines(kind, title.name, report, res.Open)))
	}
	if err := newReport(out, errOut, run, title, report, res); err != nil {
		return err
	}
	if len(report.Skipped) > 0 {
		return exitError{code: exitFindings}
	}
	return nil
}

// refuseNew prints why a name that exists is refused on stdout with exit
// 1: the person reads the file that holds it, and the command itself is
// not wrong.
func refuseNew(out io.Writer, title newTitle, cause error) error {
	if err := writeLines(out, []string{fmt.Sprintf("golaw new %s %s: refused: %v", title.kind, title.name, cause)}); err != nil {
		return err
	}
	return exitError{code: exitFindings}
}

// newReport prints the lines or the JSON document, and the person's lines
// when the output is text in the configured language.
func newReport(out, errOut io.Writer, run genRun, title newTitle, report apply.Report, res gen.Result) error {
	lines := newLines(title.kind, title.name, report, res.Open)
	if title.asJSON {
		text, err := newDocument(title, report, res.Open)
		if err != nil {
			return err
		}
		lines = []string{text}
	}
	if err := writeLines(out, lines); err != nil {
		return err
	}
	if !personal(run.policy, title.asJSON) {
		return nil
	}
	return writeLines(errOut, newPersonLines(title.kind, title.name, report, res.Open))
}
