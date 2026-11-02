package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/thirdparty"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
)

// checkFailed wraps every failure of golaw gen --check.
const checkFailed = "gen --check: %w"

// checkGen renders every entry and every pinned generator in memory and
// compares them with the checkout, writing nothing. What it cannot measure
// is exit 2, never a pass, because the pipeline is the only gate.
func (a *app) checkGen(ctx context.Context, out, errOut io.Writer, target string, opts genOptions) error {
	run, err := a.prepareGen(ctx, genDir(target))
	if err != nil {
		return fmt.Errorf(checkFailed, err)
	}
	res, err := gen.Check(ctx, run.input)
	if errors.Is(err, genkind.ErrCannotJudge) {
		if writeErr := writeLines(out, []string{checkPrefix + "unmeasured " + err.Error()}); writeErr != nil {
			return writeErr
		}
		return exitError{code: exitFailure}
	}
	if err != nil {
		return fmt.Errorf(checkFailed, err)
	}
	gens := slices.DeleteFunc(thirdparty.Generators(run.policy, run.snapshot), func(g thirdparty.Generator) bool {
		return !thirdparty.Applicable(run.snapshot, g)
	})
	third, unmeasured, err := a.bind().thirdPartyDrift(ctx, run.root, run.snapshot, gens)
	if err != nil {
		return fmt.Errorf(checkFailed, err)
	}
	drift := append(slices.Clone(res.Drift), third...)
	slices.SortStableFunc(drift, func(x, y gen.Drift) int { return strings.Compare(x.Path, y.Path) })
	if err := checkReport(out, errOut, run, opts, drift, res.Open, unmeasured); err != nil {
		return err
	}
	switch {
	case len(unmeasured) > 0:
		return exitError{code: exitFailure}
	case len(drift)+len(res.Open) > 0:
		return exitError{code: exitFindings}
	}
	return nil
}

// checkReport prints the lines or the JSON document, then the person's lines.
func checkReport(out, errOut io.Writer, run genRun, opts genOptions, drift []gen.Drift, open []holes.Hole, unmeasured []string) error {
	if opts.asJSON {
		return writeJSON(out, checkDocument(drift, open, unmeasured))
	}
	if err := writeLines(out, checkLines(drift, open, unmeasured)); err != nil {
		return err
	}
	if !personal(run.policy, false) {
		return nil
	}
	return writeLines(errOut, checkPersonLines(drift, open, unmeasured))
}

// thirdPartyDrift runs each pinned generator into a stage and compares its
// outputs with the checkout. A generator the tools module cannot provide is
// unmeasured, never a pass, and the run goes on with the others.
func (b binding) thirdPartyDrift(ctx context.Context, root string, s model.Snapshot, gens []thirdparty.Generator) ([]gen.Drift, []string, error) {
	if len(gens) == 0 {
		return nil, nil, nil
	}
	groups := driftGroups(gens)
	goBin, err := b.goBinary()
	if err != nil {
		return nil, unmeasuredAll(groups, err), nil
	}
	env := runner.UserEnv(b.environ())
	var drift []gen.Drift
	var unmeasured []string
	for _, group := range groups {
		paths, err := stagedDrift(ctx, goBin, root, s, group, env)
		if errors.Is(err, thirdparty.ErrMissing) {
			unmeasured = append(unmeasured, unmeasuredAll([][]thirdparty.Generator{group}, err)...)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		for _, p := range paths {
			drift = append(drift, gen.Drift{Path: p, Cause: gen.DriftThirdParty, Fix: gen.FixGen})
		}
	}
	return drift, unmeasured, nil
}

// unmeasuredAll names every output group unmeasured for one cause, once
// each, because the configurations of one output share the cause.
func unmeasuredAll(groups [][]thirdparty.Generator, cause error) []string {
	out := make([]string, 0, len(groups))
	for _, group := range groups {
		out = append(out, group[0].Name+": "+cause.Error())
	}
	return out
}

// driftGroups gathers, in first appearance order, the generators of one
// name that own the same outputs: the codegen configurations of one
// specification each write one file of a shared directory, so judged apart
// each would read the files of the others as missing.
func driftGroups(gens []thirdparty.Generator) [][]thirdparty.Generator {
	var groups [][]thirdparty.Generator
	for _, g := range gens {
		i := slices.IndexFunc(groups, func(group []thirdparty.Generator) bool {
			return group[0].Name == g.Name && slices.Equal(group[0].Outputs, g.Outputs)
		})
		if i < 0 {
			groups = append(groups, []thirdparty.Generator{g})
			continue
		}
		groups[i] = append(groups[i], g)
	}
	return groups
}

// stagedDrift runs one group of generators into a temporary stage, removed
// at return, and answers the outputs that differ from the checkout.
func stagedDrift(ctx context.Context, goBin, root string, s model.Snapshot, group []thirdparty.Generator, env []string) ([]string, error) {
	stage, err := os.MkdirTemp("", "golaw-gen-check-")
	if err != nil {
		return nil, fmt.Errorf("stage %s: %w", group[0].Name, err)
	}
	paths, runErr := stagedRun(ctx, goBin, root, s, group, env, stage)
	return paths, errors.Join(runErr, os.RemoveAll(stage))
}

// stagedRun stages the inputs of every generator of the group, runs each
// in the one stage and compares what they wrote together with the
// checkout; the group shares its outputs, so the first names them.
func stagedRun(ctx context.Context, goBin, root string, s model.Snapshot, group []thirdparty.Generator, env []string, stage string) ([]string, error) {
	for _, g := range group {
		if err := thirdparty.Stage(s, g, stage); err != nil {
			return nil, fmt.Errorf("stage %s: %w", g.Name, err)
		}
		bin, err := thirdparty.Resolve(ctx, goBin, root, g, env)
		if err != nil {
			return nil, fmt.Errorf("find %s: %w", g.Name, err)
		}
		if _, err = thirdparty.Exec(ctx, bin, stage, g, env); err != nil {
			return nil, fmt.Errorf("generate %s: %w", g.Config, err)
		}
	}
	paths, err := thirdparty.Compare(s, group[0], stage)
	if err != nil {
		return nil, fmt.Errorf("compare %s: %w", group[0].Name, err)
	}
	return paths, nil
}
