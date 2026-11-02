package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/apply"
	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
)

// genOptions are the flags of golaw gen.
type genOptions struct{ check, asJSON bool }

// genCommand regenerates every regenerable file from its contract (M7.8),
// or with --check proves the checkout already holds what it would write, so
// the agent edits contracts and holes only and the pipeline catches drift.
func (a *app) genCommand() *cobra.Command {
	var opts genOptions
	cmd := &cobra.Command{
		Use:   "gen [path]",
		Short: "Regenerate the generated code from its contracts, or check it with --check",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			if opts.check {
				return a.checkGen(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), target, opts)
			}
			if err := a.generate(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), target, opts); err != nil {
				return fmt.Errorf("gen: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.check, "check", false, "render in memory, compare with the checkout and write nothing")
	cmd.Flags().BoolVar(&opts.asJSON, jsonFlag, false, "print the report as JSON")
	return cmd
}

// changedOf answers the slash path of a regular file inside root, the one
// contract a run over a path regenerates from; anything else is nil, which
// means every entry. Links are resolved on both sides, because the root a
// discovery answers is the real one.
func changedOf(root, target string) []string {
	abs, err := filepath.Abs(target)
	if err != nil {
		return nil
	}
	info, err := os.Stat(abs) //nolint:gosec,nolintlint // gosec flags this taint on some runs only; the path the person named on the command line, only read for its kind
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	rel, err := filepath.Rel(realPath(root), realPath(abs))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	return []string{filepath.ToSlash(rel)}
}

// genDir is the directory the repository is discovered from: the file's
// own directory when the path names a file.
func genDir(target string) string {
	if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() {
		return filepath.Dir(target)
	}
	return target
}

// generate runs the pinned generators the path feeds, regenerates the
// entries, lands the writes and applies the safe fixers. A pending decision
// is exit 1, because the agent still has something to settle.
func (a *app) generate(ctx context.Context, out, errOut io.Writer, target string, opts genOptions) error {
	run, err := a.prepareGen(ctx, genDir(target))
	if err != nil {
		return err
	}
	changed := changedOf(run.root, target)
	bound := a.bind()
	gens, skipped := applicable(run, changed)
	notices, err := bound.runGenerators(ctx, run.root, gens)
	notices = append(skipped, notices...)
	if err != nil {
		return generatorFailed(errOut, notices, err)
	}
	if len(notices) > 0 {
		// The kinds read the outputs the generators just wrote.
		if run, err = a.prepareGen(ctx, genDir(target)); err != nil {
			return err
		}
	}
	run.input.Changed = changed
	res, err := gen.Generate(ctx, run.input)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	report, err := apply.Files(run.root, res.Writes)
	if err != nil {
		return errors.Join(fmt.Errorf("write: %w", err), writeLines(out, genLines(res, report, notices)))
	}
	notices = append(notices, bound.autofix(ctx, run.root, report.Written)...)
	if err := genReport(out, errOut, run, opts, res, report, notices); err != nil {
		return err
	}
	if len(res.Decisions) > 0 {
		return exitError{code: exitFindings}
	}
	return nil
}

// applicable answers the generators a run feeds, the ones a changed path
// triggers when the run names one, without those the checkout never fed,
// each of which is a skipped line: the same filter golaw gen --check
// applies, so the command the check names fixes what it reports.
func applicable(run genRun, changed []string) ([]thirdparty.Generator, []string) {
	gens := thirdparty.Generators(run.policy, run.snapshot)
	if changed != nil {
		gens = thirdparty.Triggered(gens, changed)
	}
	var skipped []string
	gens = slices.DeleteFunc(gens, func(g thirdparty.Generator) bool {
		if thirdparty.Applicable(run.snapshot, g) {
			return false
		}
		skipped = append(skipped, "skipped "+g.Name+" "+g.Config+": not applicable, no input of it exists and its output holds no file")
		return true
	})
	return gens, skipped
}

// generatorFailed prints the runs that succeeded and the failure, which
// quotes the generator's own output naming the contract line, with exit 2.
func generatorFailed(errOut io.Writer, notices []string, cause error) error {
	if err := writeLines(errOut, append(notices, "golaw gen: "+cause.Error())); err != nil {
		return err
	}
	return exitError{code: exitFailure}
}

// genReport prints the lines or the JSON document, then the person's lines.
func genReport(out, errOut io.Writer, run genRun, opts genOptions, res gen.Result, report apply.Report, notices []string) error {
	if opts.asJSON {
		return writeJSON(out, genDocument(res, report, notices))
	}
	if err := writeLines(out, genLines(res, report, notices)); err != nil {
		return err
	}
	if !personal(run.policy, false) {
		return nil
	}
	return writeLines(errOut, genPersonLines(res, report))
}

// writeJSON prints one indented JSON document.
func writeJSON(out io.Writer, doc any) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode the report: %w", err)
	}
	return writeLines(out, []string{string(data)})
}
