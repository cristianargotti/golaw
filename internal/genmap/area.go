package genmap

import (
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// inArea keeps what belongs to one area: its ports and use cases by name,
// the adapters of its ports, the handlers of its use cases, and the holes
// and contracts under its directories or owned by one of its entries, so
// the hole of an adapter of the area stays on the area's map.
func (r Report) inArea(area string, p model.GeneratePolicy, m *manifest.Manifest) Report {
	dirs := []string{layoutDir(p.Layout.Port, area) + "/", layoutDir(p.Layout.Usecase, area) + "/"}
	under := func(file string) bool {
		return slices.ContainsFunc(dirs, func(d string) bool { return strings.HasPrefix(file, d) })
	}
	prefix := area + "."
	out := Report{Layers: r.Layers}
	out.Ports = keep(r.Ports, func(it Item) bool { return strings.HasPrefix(it.Name, prefix) })
	out.Usecases = keep(r.Usecases, func(it Item) bool { return strings.HasPrefix(it.Name, prefix) })
	out.Adapters = keep(r.Adapters, func(it Item) bool { return hasDetail(it, "port "+prefix) })
	out.Handlers = keep(r.Handlers, func(it Item) bool { return handlesArea(it, prefix) })
	out.Contracts = keep(r.Contracts, under)
	out.Handwritten = keep(r.Handwritten, func(it Item) bool { return under(it.Path) })
	for _, h := range r.Holes {
		if under(h.Path) || ownedInArea(m, h, area, under) {
			out.Holes = append(out.Holes, h)
		}
	}
	return out
}

// keep answers the elements the predicate holds.
func keep[T any](items []T, ok func(T) bool) []T {
	var out []T
	for _, it := range items {
		if ok(it) {
			out = append(out, it)
		}
	}
	return out
}

// hasDetail reports whether a detail of the item starts with prefix.
func hasDetail(it Item, prefix string) bool {
	return slices.ContainsFunc(it.Details, func(d string) bool { return strings.HasPrefix(d, prefix) })
}

// handlesArea reports whether a handler serves a use case of the area.
func handlesArea(it Item, prefix string) bool {
	for _, d := range it.Details {
		if list, ok := strings.CutPrefix(d, "usecases "); ok {
			for name := range strings.SplitSeq(list, ", ") {
				if strings.HasPrefix(name, prefix) {
					return true
				}
			}
		}
	}
	return false
}

// ownedInArea reports whether the entry that owns the hole's file is of
// the area or is generated from a contract under the area's directories.
func ownedInArea(m *manifest.Manifest, h holes.Hole, area string, under func(string) bool) bool {
	if m == nil {
		return false
	}
	e, ok := m.Owner(h.Path)
	if !ok {
		return false
	}
	return e.Area() == area || slices.ContainsFunc(e.Contracts, func(c manifest.Contract) bool { return under(c.Path) })
}
