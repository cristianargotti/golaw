package genmap

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/model"
)

// strictServer is the interface oapi-codegen generates for the handler a
// kind fills; an assertion of it marks a handler.
const strictServer = "StrictServerInterface"

// wordPort opens a port line of the map and the port detail of an adapter.
const wordPort = "port"

// Build maps the repository: the manifest's items first, then the ones the
// sources declare that the manifest does not name, so the map is useful
// before the first golaw new and exact after it.
func Build(ctx context.Context, in Input) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, fmt.Errorf("map the repository: %w", err)
	}
	p := in.Policy
	files := goFiles(in.Snapshot)
	parsed := ports(files, p.Layout.Port)
	adapters, handlers := assertionItems(files, in)
	r := Report{
		Layers:    layers(p),
		Holes:     openHoles(files),
		Ports:     merge(manifestPorts(in.Manifest, parsed), parsed),
		Usecases:  merge(manifestItems(in, genkind.KindUsecase), parsedUsecases(files, p.Layout.Usecase)),
		Adapters:  merge(manifestItems(in, genkind.KindAdapterSQL, genkind.KindAdapterHTTP), adapters),
		Handlers:  merge(manifestItems(in, genkind.KindHandler), handlers),
		Contracts: contracts(in),
	}
	markHandwritten(&r, files)
	listHandwritten(&r, files)
	if in.Area != "" {
		r = r.inArea(in.Area, p, in.Manifest)
	}
	sortReport(&r)
	return r, nil
}

// entries answers the manifest entries of the kinds, none without one.
func entries(m *manifest.Manifest, kinds ...string) []*manifest.Entry {
	if m == nil {
		return nil
	}
	var out []*manifest.Entry
	for i := range m.Entries {
		if e := &m.Entries[i]; slices.Contains(kinds, e.Kind) {
			out = append(out, e)
		}
	}
	return out
}

// manifestPorts answers the port entries, each with the methods its parsed
// contract declares, or at its contract alone when the source holds none.
func manifestPorts(m *manifest.Manifest, parsed []Item) []Item {
	var out []Item
	for _, e := range entries(m, genkind.KindPort) {
		if i := slices.IndexFunc(parsed, func(it Item) bool { return it.Name == e.Name }); i >= 0 {
			out = append(out, parsed[i])
			continue
		}
		it := Item{Name: e.Name}
		if len(e.Contracts) > 0 {
			it.Path = e.Contracts[0].Path
		}
		out = append(out, it)
	}
	return out
}

// manifestItems answers the use case, adapter or handler entries: each at
// its first regenerable file, else its first owned file (a use case at its
// owned file, where its logic lives), with the details its kind records.
func manifestItems(in Input, kinds ...string) []Item {
	given := entries(in.Manifest, kinds...)
	out := make([]Item, 0, len(given))
	for _, e := range given {
		files := append(slices.Clone(e.Regenerable), e.Owned...)
		if e.Kind == genkind.KindUsecase {
			files = append(slices.Clone(e.Owned), e.Regenerable...)
		}
		it := Item{Name: e.Name, Details: entryDetails(e, in.Policy.Layout.Port)}
		if len(files) > 0 {
			it.Path = files[0]
		}
		out = append(out, it)
	}
	return out
}

// entryDetails are the details of one entry: the ports, input and output
// of a use case, the port of an adapter, the use cases of a handler.
func entryDetails(e *manifest.Entry, portPattern string) []string {
	var out []string
	add := func(label string, values ...string) {
		if len(values) > 0 && values[0] != "" {
			out = append(out, label+" "+strings.Join(values, ", "))
		}
	}
	switch e.Kind {
	case genkind.KindUsecase:
		add("ports", e.Ports...)
		add("in", e.In)
		add("out", e.Out)
	case genkind.KindHandler:
		add("usecases", e.Usecases...)
	default:
		for _, c := range e.Contracts {
			if area, ok := matchLayout(portPattern, path.Dir(c.Path)); ok && c.Symbol != "" {
				add(wordPort, area+"."+c.Symbol)
				break
			}
		}
	}
	return out
}

// assertionItems answers the adapters and handlers the compile time
// assertions of their directories declare.
func assertionItems(files []goFile, in Input) ([]Item, []Item) {
	l := in.Policy.Layout
	var adapters, handlers []Item
	for _, f := range files {
		dir := path.Dir(f.path)
		_, sql := matchLayout(l.AdapterSQL, dir)
		_, http := matchLayout(l.AdapterHTTP, dir)
		_, handler := matchLayout(l.Handler, dir)
		if f.test || !sql && !http && !handler {
			continue
		}
		for _, a := range assertions(f) {
			switch {
			case sql || http:
				adapters = withDetail(adapters, a.pkg+"."+a.impl, f.path, wordPort+" "+a.iface)
			case strings.HasSuffix(a.iface, "."+strictServer):
				handlers = withDetail(handlers, a.pkg+"."+a.impl, f.path, "implements "+a.iface)
			}
		}
	}
	return adapters, handlers
}

// withDetail adds a detail to the item of the name, or a new item.
func withDetail(items []Item, name, file, detail string) []Item {
	if i := slices.IndexFunc(items, func(it Item) bool { return it.Name == name }); i >= 0 {
		items[i].Details = append(items[i].Details, detail)
		return items
	}
	return append(items, Item{Name: name, Path: file, Details: []string{detail}})
}

// merge appends the parsed items whose name the manifest did not give.
func merge(given, parsed []Item) []Item {
	for _, it := range parsed {
		if !slices.ContainsFunc(given, func(g Item) bool { return g.Name == it.Name }) {
			given = append(given, it)
		}
	}
	return given
}

// contracts answers every contract path of the manifest and the OpenAPI
// document when the snapshot holds it.
func contracts(in Input) []string {
	var out []string
	if in.Manifest != nil {
		for i := range in.Manifest.Entries {
			for _, c := range in.Manifest.Entries[i].Contracts {
				out = append(out, c.Path)
			}
		}
	}
	if doc := in.Policy.ThirdParty.OpenAPI; doc != "" && in.Snapshot.Class(doc) != model.ClassUnknown {
		out = append(out, doc)
	}
	slices.Sort(out)
	return slices.Compact(out)
}
