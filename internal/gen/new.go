package gen

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// New plans golaw new (M7.8): the kind instance, the instances it needs, the
// support package on first use, the wiring and the manifest entry.
func New(ctx context.Context, in Input, kind string, req genkind.Request) (Result, error) {
	p := newPlan(in)
	k, err := p.kind(kind)
	if err != nil {
		return Result{}, err
	}
	if kind != genkind.KindSupport {
		if err := p.create(ctx, k, req, 0); err != nil {
			return Result{}, err
		}
		if err := p.wire(ctx); err != nil {
			return Result{}, err
		}
	}
	if err := p.ensureSupport(ctx); err != nil {
		return Result{}, err
	}
	return p.result(true)
}

// ensureSupport writes the parts of the support package the run's code
// uses, after every kind of the run is planned so the overlay holds that
// code; a part already in the repository is kept, so a run never shrinks it.
func (p *plan) ensureSupport(ctx context.Context) error {
	k, ok := p.lookupKind(genkind.KindSupport)
	if !ok {
		return nil
	}
	out, err := k.New(ctx, p.input(), genkind.Request{})
	if err != nil {
		return fmt.Errorf("%s: %w", genkind.KindSupport, err)
	}
	if err := p.applyRegenerable(out.Regenerable); err != nil {
		return err
	}
	p.manifest.Support = manifest.Support{Path: p.in.Policy.Support, Template: ref(k)}
	return nil
}

// create renders one new instance and the instances it needs (A56). Only a
// use case may be created again: rerunning golaw new usecase updates its
// entry, while any other repeat would overwrite owned code.
func (p *plan) create(ctx context.Context, k genkind.Kind, req genkind.Request, depth int) error {
	name := req.Name.String()
	if depth > maxNeedDepth {
		return fmt.Errorf("%w: the needs of %s %s loop", genkind.ErrRequest, k.Name(), name)
	}
	old, exists := p.manifest.Lookup(k.Name(), name)
	if exists && k.Name() != genkind.KindUsecase {
		return fmt.Errorf("%w: %s %s is in %s: %s", genkind.ErrExists, k.Name(), name, manifest.Path, firstFile(old))
	}
	out, err := k.New(ctx, p.input(), req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", k.Name(), name, err)
	}
	if !exists {
		if err := p.refuseOwned(k, name, out); err != nil {
			return err
		}
	}
	if err := p.adopt(k, out); err != nil {
		return err
	}
	return p.createNeeds(ctx, out.Needs, depth)
}

// refuseOwned stops a new instance whose owned file is already in the tree,
// naming the file, because owned code is never overwritten.
func (p *plan) refuseOwned(k genkind.Kind, name string, out genkind.Output) error {
	for _, f := range out.Owned {
		_, present, err := p.snapshot(f.Path)
		if err != nil {
			return err
		}
		if present {
			return fmt.Errorf("%w: %s %s: %s exists", genkind.ErrExists, k.Name(), name, f.Path)
		}
	}
	return nil
}

// adopt records a rendered output whose template this kit embeds.
func (p *plan) adopt(k genkind.Kind, out genkind.Output) error {
	if out.Entry.Template != ref(k) {
		return fmt.Errorf("%w: %s %s is %s, the kit embeds %s", ErrUnknownTemplate, out.Entry.Kind, out.Entry.Name, out.Entry.Template, ref(k))
	}
	p.manifest = p.manifest.With(out.Entry)
	if err := p.apply(out); err != nil {
		return err
	}
	p.touched = append(p.touched, out.Entry)
	p.done[entryKey(out.Entry.Kind, out.Entry.Name)] = true
	return nil
}

// createNeeds creates each needed instance the run and the manifest do not
// hold yet; a use case is always rendered again so it gains the new port.
func (p *plan) createNeeds(ctx context.Context, needs []genkind.Need, depth int) error {
	for i := range needs {
		need := &needs[i]
		nk, err := p.kind(need.Kind)
		if err != nil {
			return err
		}
		name := need.Request.Name.String()
		if p.done[entryKey(need.Kind, name)] {
			continue
		}
		if _, ok := p.manifest.Lookup(need.Kind, name); ok && need.Kind != genkind.KindUsecase {
			continue
		}
		if err := p.create(ctx, nk, need.Request, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// firstFile is the file an existing entry is named by in a refusal.
func firstFile(e manifest.Entry) string {
	if len(e.Owned) > 0 {
		return e.Owned[0]
	}
	if len(e.Regenerable) > 0 {
		return e.Regenerable[0]
	}
	return manifest.Path
}
