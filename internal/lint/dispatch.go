package lint

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// Options is one run of the lint target over a repository (M6.9).
type Options struct {
	Root     string // the repository root the tools run in
	Snapshot model.Snapshot
	Policy   *model.EffectivePolicy
	Scope    surface.Scope // the analyzer scope: a file outside it is never linted
	Set      string        // empty: Policy.Surface.Rules
	// Only names tools or surface languages; empty runs every tool the
	// inventory needs.
	Only     []string
	Resolver toolchain.Resolver
	Kit      string // the kit version, for toolchain.ConfigDir
}

// selection is one driver of a run and the languages whose files it reads.
type selection struct {
	driver    Driver
	languages []surface.Language
}

// Drivers answers the drivers of every surface sorted by tool, with the
// public web sets; the drivers of one tool keep their part's order, so ruff
// check runs before ruff format.
func Drivers() []Driver {
	drivers := slices.Concat(webDrivers(), pythonDrivers(), infraDrivers())
	slices.SortStableFunc(drivers, func(a, b Driver) int { return strings.Compare(a.Tool, b.Tool) })
	return drivers
}

// driversFor answers the drivers of a policy: the kit's drivers in their
// order, with the web drivers a pinned profile's Sonar way snapshot
// governs in place of the public ones.
func driversFor(policy *model.EffectivePolicy) []Driver {
	drivers := Drivers()
	web := webDriversFor(policy)
	for i := range drivers {
		if j := slices.IndexFunc(web, func(d Driver) bool { return d.Tool == drivers[i].Tool }); j >= 0 {
			drivers[i] = web[j]
		}
	}
	return drivers
}

// Run lints the files of the analyzer scope with the pinned tools and the
// kit configurations, one outcome per tool. A tool it cannot start, an
// output it cannot read and a file the inventory could not judge are
// Unmeasured, never clean.
func Run(ctx context.Context, o Options) (*engine.Run, error) {
	set := o.Set
	if set == "" {
		set = surfacePolicy(o.Policy).Rules
	}
	if set != model.SurfaceParity && set != model.SurfaceStrict {
		return nil, fmt.Errorf("%w: surface set %q: want parity or strict", model.ErrUsage, set)
	}
	chosen, err := selected(driversFor(o.Policy), o.Only)
	if err != nil {
		return nil, err
	}
	inv, err := surface.Take(ctx, o.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("lint the surface: %w", err)
	}
	run := &engine.Run{Mode: model.ModeScan}
	if reasons := unjudged(inv, o.Scope); len(reasons) > 0 {
		run.Outcomes = append(run.Outcomes, engine.Outcome{
			Spec:   spec("inventory"),
			Result: model.Result{Status: model.Unmeasured(strings.Join(reasons, "; "))},
		})
	}
	for _, group := range byTool(chosen) {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("lint the surface: %w", err)
		}
		results := make([]model.Result, 0, len(group))
		for _, s := range group {
			results = append(results, runDriver(ctx, o, set, inv, s.driver, s.languages))
		}
		run.Outcomes = append(run.Outcomes, engine.Outcome{Spec: spec(group[0].driver.Tool), Result: merge(results)})
	}
	return run, nil
}

// surfacePolicy answers the surface block of a policy, the default one
// when the caller holds none.
func surfacePolicy(p *model.EffectivePolicy) model.SurfacePolicy {
	if p == nil {
		return model.DefaultSurface()
	}
	return p.Surface
}

// selected answers the drivers --only names, each with the languages it
// reads: a tool name keeps all of its languages, a language name narrows a
// tool of two languages to that one.
func selected(drivers []Driver, only []string) ([]selection, error) {
	if len(only) == 0 {
		out := make([]selection, 0, len(drivers))
		for _, d := range drivers {
			out = append(out, selection{driver: d, languages: d.Languages})
		}
		return out, nil
	}
	for _, name := range only {
		known := slices.ContainsFunc(drivers, func(d Driver) bool { return d.Tool == name }) ||
			slices.Contains(surface.Languages(), surface.Language(name))
		if !known {
			return nil, fmt.Errorf("%w: --only %s names no tool and no surface language", model.ErrUsage, name)
		}
	}
	var out []selection
	for _, d := range drivers {
		if slices.Contains(only, d.Tool) {
			out = append(out, selection{driver: d, languages: d.Languages})
			continue
		}
		named := slices.DeleteFunc(slices.Clone(d.Languages), func(l surface.Language) bool { return !slices.Contains(only, string(l)) })
		if len(named) > 0 {
			out = append(out, selection{driver: d, languages: named})
		}
	}
	return out, nil
}

// byTool groups the selections of one tool, keeping the driver order.
func byTool(chosen []selection) [][]selection {
	var groups [][]selection
	for _, s := range chosen {
		if n := len(groups); n > 0 && groups[n-1][0].driver.Tool == s.driver.Tool {
			groups[n-1] = append(groups[n-1], s)
			continue
		}
		groups = append(groups, []selection{s})
	}
	return groups
}

// unjudged answers the inventory entries a linter would read: a fixture
// under testdata or a file outside the analyzer scope is never linted, so
// the detector's doubt about it never fails the run.
func unjudged(inv surface.Inventory, scope surface.Scope) []string {
	var out []string
	for _, entry := range inv.Unmeasured {
		path, _, _ := strings.Cut(entry, ": ")
		if slices.Contains(strings.Split(path, "/"), "testdata") || scope.Role(path) == surface.RoleOutside {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// merge joins the results of the drivers of one tool into one: Unmeasured
// when any is, Measured when any is, NotApplicable otherwise.
func merge(results []model.Result) model.Result {
	var merged model.Result
	var unmeasured []string
	measured := false
	for _, r := range results {
		merged.Findings = append(merged.Findings, r.Findings...)
		switch r.Status.Kind {
		case model.StatusUnmeasured:
			unmeasured = append(unmeasured, r.Status.Reason)
		case model.StatusMeasured:
			measured = true
			merged.Scope = r.Scope
		case model.StatusNotApplicable:
		}
	}
	model.SortFindings(merged.Findings)
	switch {
	case len(unmeasured) > 0:
		merged.Status = model.Unmeasured(strings.Join(unmeasured, "; "))
	case measured:
		merged.Status = model.Measured()
	default:
		merged.Status = results[0].Status
	}
	return merged
}
