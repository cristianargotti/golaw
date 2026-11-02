package handler

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// kind is the handler kind; it holds no state, every run reads its Input.
type kind struct{}

// Kind answers the handler kind of phase M7.1, which implements the strict
// server interface the pinned oapi-codegen writes from the OpenAPI document.
func Kind() genkind.Kind { return kind{} }

// Name answers the kind name of headers and the manifest.
func (kind) Name() string { return genkind.KindHandler }

// Template answers the template version this kit embeds.
func (kind) Template() int { return version }

// New renders a handler over use cases and ports that must exist already,
// which newView checks, so a handler asks no other kind for anything.
func (kind) New(_ context.Context, in genkind.Input, req genkind.Request) (genkind.Output, error) {
	v, err := newView(in, req)
	if err != nil {
		return genkind.Output{}, err
	}
	regen, err := regenerable(v, in.Version)
	if err != nil {
		return genkind.Output{}, err
	}
	own, err := owned(v)
	if err != nil {
		return genkind.Output{}, err
	}
	return genkind.Output{Entry: entryOf(v, filePaths(regen), filePaths(own)), Regenerable: regen, Owned: own, Bindings: bindingsOf(v)}, nil
}

// Regenerate rewrites the regenerable part from the document and only
// appends to the owned part (M7.4.2 rule 5). The context is unused because
// the kind is pure over its input; the engine owns cancellation.
func (kind) Regenerate(_ context.Context, in genkind.Input, entry manifest.Entry) (genkind.Output, error) {
	v, err := entryView(in, entry)
	if err != nil {
		return genkind.Output{}, err
	}
	regen, err := regenerable(v, in.Version)
	if err != nil {
		return genkind.Output{}, err
	}
	r, err := reconcile(v, in.Snapshot, entry.Owned)
	if err != nil {
		return genkind.Output{}, err
	}
	return genkind.Output{
		Entry:       entryOf(v, filePaths(regen), r.Paths),
		Regenerable: regen, Owned: r.Owned, Appends: r.Appends, Decisions: r.Decisions, Bindings: bindingsOf(v),
	}, nil
}

// entryView reads the server file the entry was made from, so a
// regeneration never follows a server path the policy moved since.
func entryView(in genkind.Input, e manifest.Entry) (view, error) {
	if e.Kind != genkind.KindHandler || e.Type() != handlerType {
		return view{}, fmt.Errorf("%w: entry %s is not a handler", manifest.ErrManifest, e.Name)
	}
	var api, server string
	for _, c := range e.Contracts {
		if strings.HasSuffix(c.Path, ".go") {
			server = c.Path
		} else {
			api = c.Path
		}
	}
	if api == "" || server == "" {
		return view{}, fmt.Errorf("%w: handler entry %s lists no OpenAPI document and server file in its contracts", manifest.ErrManifest, e.Name)
	}
	return build(in, e.Area(), api, server, e.Usecases)
}

func entryOf(v view, regenerable, owned []string) manifest.Entry {
	contracts := []manifest.Contract{{Path: v.Server}, {Path: v.API}}
	slices.SortFunc(contracts, func(a, b manifest.Contract) int { return strings.Compare(a.Path, b.Path) })
	areas := make([]string, len(v.Areas))
	for i, a := range v.Areas {
		areas[i] = a.Name
	}
	return manifest.Entry{
		Kind: genkind.KindHandler, Template: genheader.Header{Kind: genkind.KindHandler, Template: version}.Ref(),
		Name: v.Package + "." + handlerType, Contracts: contracts, Usecases: areas, Regenerable: regenerable, Owned: owned,
	}
}

// bindingsOf answers one binding per area in the order of the parameters
// of NewHandler, because the wiring kind passes the area variables of the
// bindings of one call in that order.
func bindingsOf(v view) []genkind.Binding {
	out := make([]genkind.Binding, len(v.Areas))
	for i, a := range v.Areas {
		out[i] = genkind.Binding{Kind: genkind.KindHandler, Area: a.Name, Import: v.Self, Call: v.Package + ".NewHandler"}
	}
	return out
}

func filePaths(files []genkind.File) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Path
	}
	return out
}
