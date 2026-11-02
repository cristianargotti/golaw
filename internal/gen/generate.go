package gen

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// Generate plans golaw gen and the post hook: the regenerable writes that
// differ, the stubs owned files lack and the decisions a gone contract asks
// for. A hook event names its changed contracts, so only their entries
// render, inside the hook budget; no change named means every entry.
func Generate(ctx context.Context, in Input) (Result, error) {
	p := newPlan(in)
	full := len(in.Changed) == 0
	if err := p.run(ctx, full); err != nil {
		return Result{}, err
	}
	return p.result(full)
}

// run renders the entries of the run; a full run also renders the support
// package and the composition root, which every entry feeds.
func (p *plan) run(ctx context.Context, full bool) error {
	if full {
		if err := p.remove(); err != nil {
			return err
		}
	}
	entries := p.entries(full)
	for i := range entries {
		if entries[i].Kind == genkind.KindWiring {
			continue
		}
		if err := p.regenerate(ctx, entries[i], full); err != nil {
			return err
		}
	}
	if !full {
		return nil
	}
	if err := p.regenerateSupport(ctx); err != nil {
		return err
	}
	return p.wire(ctx)
}

// entries answers every entry of a full run, else those generated from a
// changed contract, each once.
func (p *plan) entries(full bool) []manifest.Entry {
	if full {
		return slices.Clone(p.manifest.Entries)
	}
	seen := map[string]bool{}
	var out []manifest.Entry
	for _, changed := range p.in.Changed {
		contracting := p.manifest.Contracting(changed)
		for i := range contracting {
			if key := entryKey(contracting[i].Kind, contracting[i].Name); !seen[key] {
				seen[key] = true
				out = append(out, contracting[i])
			}
		}
	}
	return out
}

// regenerate renders one existing entry. A gone contract is a decision and
// the entry is left as written, because rendering it would delete code the
// agent may still want (M7.9.4).
func (p *plan) regenerate(ctx context.Context, e manifest.Entry, full bool) error {
	k, err := p.entryKind(e)
	if err != nil {
		return err
	}
	p.done[entryKey(e.Kind, e.Name)] = true
	gone, err := p.missingContract(e)
	if err != nil {
		return err
	}
	if gone != "" {
		p.gone = append(p.gone, goneContract{path: gone, entry: e})
		p.decisions = append(p.decisions, genkind.Decision{Path: gone, Text: fmt.Sprintf(
			"contract %s of %s %s is gone: restore it, or delete the owned files %s and run golaw gen to remove the entry",
			gone, e.Kind, e.Name, strings.Join(e.Owned, ", "))})
		return nil
	}
	out, err := k.Regenerate(ctx, p.input(), e)
	if err != nil {
		return fmt.Errorf("%s %s: %w", e.Kind, e.Name, err)
	}
	if err := p.apply(out); err != nil {
		return err
	}
	if full && out.Entry.Name == e.Name && !slices.Equal(out.Entry.Regenerable, e.Regenerable) {
		p.manifest = p.manifest.With(out.Entry)
	}
	p.touched = append(p.touched, e)
	return nil
}

// entryKind answers the kind of an entry whose template this kit embeds.
func (p *plan) entryKind(e manifest.Entry) (genkind.Kind, error) {
	k, err := p.kind(e.Kind)
	if err != nil {
		return nil, fmt.Errorf("entry %s: %w", e.Name, err)
	}
	if e.Template != ref(k) {
		return nil, fmt.Errorf("%w: %s %s is %s, the kit embeds %s", ErrUnknownTemplate, e.Kind, e.Name, e.Template, ref(k))
	}
	return k, nil
}

// regenerateSupport renders the support package the manifest names.
func (p *plan) regenerateSupport(ctx context.Context) error {
	support := p.manifest.Support
	if support.Path == "" {
		return nil
	}
	if _, ok := p.lookupKind(genkind.KindSupport); !ok {
		return nil
	}
	e := manifest.Entry{Kind: genkind.KindSupport, Template: support.Template, Name: path.Base(support.Path) + ".Support"}
	k, err := p.entryKind(e)
	if err != nil {
		return err
	}
	out, err := k.Regenerate(ctx, p.input(), e)
	if err != nil {
		return fmt.Errorf("%s: %w", genkind.KindSupport, err)
	}
	return p.apply(out)
}

// wire renders the composition root from the bindings of every entry, so a
// new adapter is bound without the agent editing main.
func (p *plan) wire(ctx context.Context) error {
	w := p.wirer()
	if w == nil {
		return nil
	}
	bindings, err := p.otherBindings(ctx)
	if err != nil {
		return err
	}
	// A root with nothing to bind would only open its own infra hole, so it
	// is rendered once some entry binds a value, and kept once it exists.
	wired := slices.ContainsFunc(p.manifest.Entries, func(e manifest.Entry) bool { return e.Kind == genkind.KindWiring })
	if len(bindings) == 0 && !wired {
		return nil
	}
	if p.in.Main == MainByHand {
		p.decisions = append(p.decisions, handWired(bindings)...)
		return nil
	}
	out, err := w.Wire(ctx, p.input(), p.in.Main, bindings)
	if err != nil {
		return fmt.Errorf("%s: %w", genkind.KindWiring, err)
	}
	return p.adopt(w, out)
}

// handWired turns each binding into the line the agent writes in the hand
// written composition root, because a second, generated root beside it
// would leave an infra hole open that gen --check fails on.
func handWired(bindings []genkind.Binding) []genkind.Decision {
	out := make([]genkind.Decision, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, genkind.Decision{
			Path: "golaw.yml", Line: 1,
			Text: "wire " + b.Call + " (" + b.Import.Path + ") into the hand written composition root by hand",
		})
	}
	return out
}

// otherBindings adds to the bindings the run rendered those of the entries
// it did not, so a golaw new run wires the whole root and not one entry.
func (p *plan) otherBindings(ctx context.Context) ([]genkind.Binding, error) {
	bindings := slices.Clone(p.bindings)
	for i := range p.manifest.Entries {
		e := &p.manifest.Entries[i]
		if e.Kind == genkind.KindWiring || p.done[entryKey(e.Kind, e.Name)] {
			continue
		}
		more, err := p.bindingsOf(ctx, *e)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, more...)
	}
	return bindings, nil
}

// bindingsOf renders an entry the run did not touch for its bindings alone;
// an entry whose contract is gone binds nothing until it is restored.
func (p *plan) bindingsOf(ctx context.Context, e manifest.Entry) ([]genkind.Binding, error) {
	k, err := p.entryKind(e)
	if err != nil {
		return nil, err
	}
	gone, err := p.missingContract(e)
	if err != nil || gone != "" {
		return nil, err
	}
	out, err := k.Regenerate(ctx, p.input(), e)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", e.Kind, e.Name, err)
	}
	return out.Bindings, nil
}

// result closes the plan: the manifest write, the writes in path order and
// the holes of the owned files before and after the run.
func (p *plan) result(withManifest bool) (Result, error) {
	if withManifest {
		if err := p.putManifest(); err != nil {
			return Result{}, err
		}
	}
	res := Result{Decisions: p.decisions, Entries: p.touched, Removed: p.removed, Manifest: p.manifest}
	for _, w := range p.writes {
		res.Writes = append(res.Writes, w)
	}
	slices.SortFunc(res.Writes, func(a, b Write) int { return strings.Compare(a.Path, b.Path) })
	var before, after []holes.Hole
	for _, owned := range slices.Sorted(maps.Keys(p.owned)) {
		was, err := p.holesOf(owned, p.snapshot)
		if err != nil {
			return Result{}, err
		}
		now, err := p.holesOf(owned, p.current)
		if err != nil {
			return Result{}, err
		}
		before, after = append(before, was...), append(after, now...)
	}
	res.Open = sortHoles(after)
	res.Opened, res.Closed = holes.Diff(before, after)
	return res, nil
}

// holesOf answers the open holes of one owned file as read, none when absent.
func (p *plan) holesOf(rel string, read func(string) ([]byte, bool, error)) ([]holes.Hole, error) {
	content, present, err := read(rel)
	if err != nil || !present {
		return nil, err
	}
	found, err := holes.Find(rel, content)
	if err != nil {
		return nil, fmt.Errorf("holes of %s: %w", rel, err)
	}
	return found, nil
}

func sortHoles(hs []holes.Hole) []holes.Hole {
	slices.SortFunc(hs, func(a, b holes.Hole) int {
		return cmp.Or(strings.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	return hs
}
