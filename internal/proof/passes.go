package proof

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/cristianargotti/golaw/internal/engine"
	"github.com/cristianargotti/golaw/internal/model"
)

// The names of the four passes.
const (
	PassCompleteness = "completeness"
	PassBaseline     = "baseline"
	PassDeletion     = "deletion"
	PassOverFire     = "over-fire"
)

// Failure is one broken proof.
type Failure struct {
	Pass   string
	ID     string
	Reason string
}

// String renders the failure for the test log.
func (f Failure) String() string { return fmt.Sprintf("%s %s: %s", f.Pass, f.ID, f.Reason) }

// Completeness compares the manifest with the registry both ways, so that
// deleting a registration or a manifest entry alone is red.
func Completeness(reg *engine.Registry, m Manifest) []Failure {
	var out []Failure
	ids, names := reg.IDs(), m.Names()
	for _, id := range ids {
		if !slices.Contains(names, id) {
			out = append(out, Failure{Pass: PassCompleteness, ID: id, Reason: "registered check has no manifest entry"})
		}
	}
	for _, name := range names {
		if !slices.Contains(ids, name) {
			out = append(out, Failure{Pass: PassCompleteness, ID: name, Reason: "manifest entry has no registered check"})
		}
	}
	return out
}

// Verify runs every pass and returns every failure, empty when the proof
// holds.
func Verify(ctx context.Context, reg *engine.Registry, root string, m Manifest) []Failure {
	out := Completeness(reg, m)
	out = append(out, Baseline(ctx, reg, root, m)...)
	out = append(out, Deletion(ctx, reg, root, m)...)
	return append(out, OverFire(ctx, reg, root, m)...)
}

// Baseline proves, through production dispatch, that the red fixture of
// every entry yields its code from its check and that the near miss does
// not, both with the check measured.
func Baseline(ctx context.Context, reg *engine.Registry, root string, m Manifest) []Failure {
	return eachEntry(reg, root, m, PassBaseline, func(p pair) []Failure { return p.baseline(ctx, reg) })
}

// Deletion proves that the red case depends on its check: with the check
// replaced by a no-op, no other check may still report the code.
func Deletion(ctx context.Context, reg *engine.Registry, root string, m Manifest) []Failure {
	return eachEntry(reg, root, m, PassDeletion, func(p pair) []Failure { return p.deletion(ctx, reg) })
}

// OverFire proves that the near miss reaches its check: with the check
// replaced by one that always reports, the near miss must turn red.
func OverFire(ctx context.Context, reg *engine.Registry, root string, m Manifest) []Failure {
	return eachEntry(reg, root, m, PassOverFire, func(p pair) []Failure { return p.overFire(ctx, reg) })
}

// eachEntry loads every entry and proves it; an entry that cannot be
// loaded is a failure of the pass, never a skip.
func eachEntry(reg *engine.Registry, root string, m Manifest, pass string, prove func(pair) []Failure) []Failure {
	var out []Failure
	for _, e := range m.Checks {
		p, err := load(reg, root, e)
		if err != nil {
			out = append(out, Failure{Pass: pass, ID: e.ID, Reason: err.Error()})
			continue
		}
		out = append(out, prove(p)...)
	}
	return out
}

// pair is one entry with its spec and its two loaded fixtures.
type pair struct {
	entry Entry
	spec  model.Spec
	red   Fixture
	near  Fixture
}

func load(reg *engine.Registry, root string, e Entry) (pair, error) {
	c, ok := reg.Lookup(e.ID)
	if !ok {
		return pair{}, fmt.Errorf("%w: %s is not registered", ErrManifest, e.ID)
	}
	red, err := LoadFixture(filepath.Join(root, filepath.FromSlash(e.Red)))
	if err != nil {
		return pair{}, err
	}
	near, err := LoadFixture(filepath.Join(root, filepath.FromSlash(e.Near)))
	if err != nil {
		return pair{}, err
	}
	return pair{entry: e, spec: c.Spec(), red: red, near: near}, nil
}

// verdict is what one dispatch says about the entry's check.
type verdict struct {
	status model.Status // the status of the entry's check
	own    bool         // the entry's check reported the code
	other  string       // another check that reported the code, or empty
}

func (p pair) run(ctx context.Context, reg *engine.Registry, f Fixture) (verdict, error) {
	in, err := f.Input(p.spec.Scope, Policy())
	if err != nil {
		return verdict{}, err
	}
	run, err := engine.Dispatch(ctx, reg, in)
	if err != nil {
		return verdict{}, fmt.Errorf("dispatch: %w", err)
	}
	var v verdict
	for i := range run.Outcomes {
		o := &run.Outcomes[i]
		if o.Spec.ID == p.entry.ID {
			v.status = o.Result.Status
		}
		for _, f := range o.Result.Findings {
			if f.Code != p.entry.Code {
				continue
			}
			if o.Spec.ID == p.entry.ID {
				v.own = true
			} else if v.other == "" {
				v.other = o.Spec.ID
			}
		}
	}
	return v, nil
}

func (p pair) baseline(ctx context.Context, reg *engine.Registry) []Failure {
	var out []Failure
	fail := func(reason string) { out = append(out, Failure{Pass: PassBaseline, ID: p.entry.ID, Reason: reason}) }
	red, err := p.run(ctx, reg, p.red)
	switch {
	case err != nil:
		fail("red fixture: " + err.Error())
	case red.status.Kind != model.StatusMeasured:
		fail(fmt.Sprintf("red fixture: the check was %s: %s", red.status.Kind, red.status.Reason))
	case !red.own:
		fail("red fixture yields no finding with code " + p.entry.Code)
	}
	near, err := p.run(ctx, reg, p.near)
	switch {
	case err != nil:
		fail("near miss: " + err.Error())
	case near.status.Kind != model.StatusMeasured:
		fail(fmt.Sprintf("near miss: the check was %s: %s", near.status.Kind, near.status.Reason))
	case near.own:
		fail(fmt.Sprintf("near miss yields %s: the check over-fires", p.entry.Code))
	}
	return out
}

func (p pair) deletion(ctx context.Context, reg *engine.Registry) []Failure {
	fail := func(reason string) []Failure { return []Failure{{Pass: PassDeletion, ID: p.entry.ID, Reason: reason}} }
	neutral, err := Rewire(reg, p.entry.ID, NoOp)
	if err != nil {
		return fail(err.Error())
	}
	v, err := p.run(ctx, neutral, p.red)
	switch {
	case err != nil:
		return fail("red fixture: " + err.Error())
	case v.own:
		return fail("the no-op replacement still reports: rewiring failed")
	case v.other != "":
		return fail(fmt.Sprintf("with the check neutralized %s still reports %s: the red case does not depend on the check", v.other, p.entry.Code))
	}
	return nil
}

func (p pair) overFire(ctx context.Context, reg *engine.Registry) []Failure {
	fail := func(reason string) []Failure { return []Failure{{Pass: PassOverFire, ID: p.entry.ID, Reason: reason}} }
	loud, err := Rewire(reg, p.entry.ID, func(c model.Check) model.Check { return AlwaysFinding(c, p.entry.Code) })
	if err != nil {
		return fail(err.Error())
	}
	v, err := p.run(ctx, loud, p.near)
	switch {
	case err != nil:
		return fail("near miss: " + err.Error())
	case !v.own:
		return fail(fmt.Sprintf("with the check over-firing the near miss stays clean: the fixture never reaches the check (%s: %s)", v.status.Kind, v.status.Reason))
	}
	return nil
}
