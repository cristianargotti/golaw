package main

import (
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// contractLabels names the port a contract declares, so the summary reads
// which interface was regenerated.
func contractLabels(m manifest.Manifest, contracts []string) map[string]string {
	labels := make(map[string]string, len(contracts))
	for _, p := range contracts {
		var symbols []string
		entries := m.Contracting(p)
		for j := range entries {
			e := &entries[j]
			i := slices.IndexFunc(e.Contracts, func(c manifest.Contract) bool { return c.Path == p && c.Symbol != "" })
			if i >= 0 && !slices.Contains(symbols, e.Contracts[i].Symbol) {
				symbols = append(symbols, e.Contracts[i].Symbol)
			}
		}
		switch len(symbols) {
		case 0:
		case 1:
			labels[p] = " (port " + symbols[0] + ")"
		default:
			labels[p] = " (ports " + strings.Join(symbols, ", ") + ")"
		}
	}
	return labels
}

// hookGenInput is the engine input of a post event over the snapshot.
func hookGenInput(snap model.Snapshot, pol *model.EffectivePolicy, m manifest.Manifest, changed []string) (gen.Input, error) {
	module, err := moduleOf(snap)
	if err != nil {
		return gen.Input{}, err
	}
	return gen.Input{
		Module: module, Version: pol.KitVersion, Policy: pol.Generate,
		Snapshot: snap, Manifest: m, Contracts: contract.Reader{Snapshot: snap},
		Kinds: kinds.All(), Changed: changed, Main: wiringMain(m),
	}, nil
}

// wiringMain is the main the manifest already wires. A manifest without a
// wiring entry answers gen.MainByHand, so the hook never writes a second
// composition root at the repository root: it names the bindings instead.
func wiringMain(m manifest.Manifest) string {
	for i := range m.Entries {
		e := &m.Entries[i]
		if e.Kind != genkind.KindWiring {
			continue
		}
		if len(e.Regenerable) > 0 && path.Dir(e.Regenerable[0]) == "." {
			return ""
		}
		return e.Area()
	}
	return gen.MainByHand
}
