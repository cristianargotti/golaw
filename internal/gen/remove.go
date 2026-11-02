package gen

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// remove drops from the manifest each entry whose owned files the person
// deleted and that no remaining entry needs, and deletes its regenerable
// files: deleting the owned code is how a person asks for an entry to go,
// and the engine itself never deletes owned code (A40). Only a full run
// removes, because a run over changed paths never sees every entry.
func (p *plan) remove() error {
	var candidates []manifest.Entry
	rest := manifest.Manifest{Schema: p.manifest.Schema, Support: p.manifest.Support}
	for i := range p.manifest.Entries {
		e := p.manifest.Entries[i]
		gone, err := p.ownedGone(e)
		if err != nil {
			return err
		}
		if gone {
			candidates = append(candidates, e)
			continue
		}
		rest = rest.With(e)
	}
	for i := range candidates {
		e := candidates[i]
		if needed(rest, e) {
			rest = rest.With(e)
			continue
		}
		for _, rel := range e.Regenerable {
			if err := p.deleteRegenerable(rel); err != nil {
				return err
			}
		}
		p.removed = append(p.removed, e)
	}
	p.manifest = rest
	return nil
}

// ownedGone reports an entry other than support and wiring that owns files
// and whose every owned file the snapshot lacks.
func (p *plan) ownedGone(e manifest.Entry) (bool, error) {
	if e.Kind == genkind.KindSupport || e.Kind == genkind.KindWiring || len(e.Owned) == 0 {
		return false, nil
	}
	for _, rel := range e.Owned {
		_, present, err := p.snapshot(rel)
		if err != nil || present {
			return false, err
		}
	}
	return true, nil
}

// needed reports an entry another entry of m still relies on: a port while
// a use case of its area lists its type, because the use case's code calls
// it; no other kind is needed by another entry.
func needed(m manifest.Manifest, e manifest.Entry) bool {
	if e.Kind != genkind.KindPort {
		return false
	}
	return slices.ContainsFunc(m.Entries, func(u manifest.Entry) bool {
		return u.Kind == genkind.KindUsecase && u.Area() == e.Area() && slices.Contains(u.Ports, e.Type())
	})
}

// deleteRegenerable asks apply.Files to delete one regenerable file the
// snapshot holds, bound to the bytes the run read.
func (p *plan) deleteRegenerable(rel string) error {
	_, present, err := p.snapshot(rel)
	if err != nil || !present {
		return err
	}
	return p.put(rel, nil, WriteDelete)
}
