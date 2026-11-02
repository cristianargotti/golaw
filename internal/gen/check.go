package gen

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
)

// Check plans golaw gen --check: every entry renders, whatever the event
// changed, and the writes a golaw gen would land become drift instead; the
// check itself writes nothing, so it is safe in any pipeline.
func Check(ctx context.Context, in Input) (Result, error) {
	p := newPlan(in)
	if err := p.run(ctx, true); err != nil {
		return Result{}, err
	}
	if err := p.putManifest(); err != nil {
		return Result{}, err
	}
	res := Result{Drift: p.drift(), Decisions: p.decisionsBesideGone(), Entries: p.touched, Removed: p.removed, Manifest: p.manifest}
	var open []holes.Hole
	for i := range p.manifest.Entries {
		for _, owned := range p.manifest.Entries[i].Owned {
			found, err := p.holesOf(owned, p.snapshot)
			if err != nil {
				return Result{}, err
			}
			open = append(open, found...)
		}
	}
	res.Open = sortHoles(open)
	return res, nil
}

// drift translates each pending write into the difference it would fix, in
// path order; a gone contract is restored from git, never regenerated.
func (p *plan) drift() []Drift {
	out := make([]Drift, 0, len(p.writes)+len(p.gone))
	for _, path := range slices.Sorted(maps.Keys(p.writes)) {
		w := p.writes[path]
		cause := DriftMissing
		switch {
		case w.Mode == WriteDelete:
			cause = DriftRemoved
		case p.stubs[path] && w.Mode == WriteAppend:
			cause = DriftStub
		case w.Mode != WriteOwned && w.Base != "":
			cause = DriftDiffers
		}
		out = append(out, Drift{Path: path, Cause: cause, Fix: FixGen})
	}
	for i := range p.gone {
		gone := p.gone[i].path
		out = append(out, Drift{Path: gone, Cause: DriftContractGone, Fix: "git restore " + gone})
	}
	if len(out) == 0 {
		return nil
	}
	slices.SortStableFunc(out, func(a, b Drift) int { return strings.Compare(a.Path, b.Path) })
	return out
}

// decisionsBesideGone drops the decision of each gone contract, which the
// drift already reports with its fix.
func (p *plan) decisionsBesideGone() []genkind.Decision {
	var out []genkind.Decision
	for _, d := range p.decisions {
		if !slices.ContainsFunc(p.gone, func(g goneContract) bool { return g.path == d.Path }) {
			out = append(out, d)
		}
	}
	return out
}
