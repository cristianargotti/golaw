package usecase

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// kind is the use case kind; it holds no state, every run reads its Input.
type kind struct{}

// Kind answers the use case kind (contract 2.7).
func Kind() genkind.Kind { return kind{} }

// Name answers the kind name of headers and the manifest.
func (kind) Name() string { return genkind.KindUsecase }

// Template answers the template version this kit embeds.
func (kind) Template() int { return templateVersion }

// New renders a use case from the request, or updates the ports of the
// entry it names, and asks for every port that has no entry yet (A56).
func (kind) New(_ context.Context, in genkind.Input, req genkind.Request) (genkind.Output, error) {
	if err := validate(req); err != nil {
		return genkind.Output{}, err
	}
	dir, err := directory(in.Policy.Layout, req.Name.Area)
	if err != nil {
		return genkind.Output{}, err
	}
	e, err := resolve(in.Manifest, req, dir)
	if err != nil {
		return genkind.Output{}, err
	}
	all, err := siblings(in.Manifest, e)
	if err != nil {
		return genkind.Output{}, err
	}
	pkg, err := scanNew(in, dir, req.Name)
	if err != nil {
		return genkind.Output{}, err
	}
	out, err := files(in, dir, newView(pkg, e, all), e)
	if err != nil {
		return genkind.Output{}, err
	}
	out.Needs = needs(in.Manifest, req, e)
	return out, nil
}

// scanNew reads the consuming package of a new use case and refuses an
// area whose wiring is written by hand.
func scanNew(in genkind.Input, dir string, name genkind.Name) (areaPackage, error) {
	pkg, err := scan(in.Snapshot, dir, name.Area)
	if err != nil {
		return areaPackage{}, err
	}
	return pkg, handWritten(in.Manifest, pkg, name)
}

// handWritten refuses an area the kit has never generated whose package
// declares an area name, because the generated area file beside it would
// redeclare the name and stop the package build.
func handWritten(m manifest.Manifest, pkg areaPackage, name genkind.Name) error {
	if slices.ContainsFunc(m.Entries, func(e manifest.Entry) bool {
		return e.Kind == genkind.KindUsecase && e.Area() == name.Area
	}) {
		return nil
	}
	for _, area := range areaNames() {
		if file, ok := pkg.Generated[area]; ok {
			return fmt.Errorf("%w: %s declares %s by hand in %s: golaw writes Deps, UseCases and NewUseCases for an area it generates, so create the use case in an area of its own (golaw new usecase <area>.%s) or rename the hand written %s", genkind.ErrRequest, name.Area, area, file, name.Type, area)
		}
	}
	return nil
}

// Regenerate renders the regenerable files of an entry again; a port the
// package no longer declares is a decision for the agent, since dropping
// it from the use case would change owned code the kit never edits.
func (kind) Regenerate(_ context.Context, in genkind.Input, entry manifest.Entry) (genkind.Output, error) {
	dir, err := directory(in.Policy.Layout, entry.Area())
	if err != nil {
		return genkind.Output{}, err
	}
	all, err := siblings(in.Manifest, entry)
	if err != nil {
		return genkind.Output{}, err
	}
	pkg, err := scan(in.Snapshot, dir, entry.Area())
	if err != nil {
		return genkind.Output{}, err
	}
	out, err := files(in, dir, newView(pkg, entry, all), entry)
	if err != nil {
		return genkind.Output{}, err
	}
	out.Decisions = missingPorts(pkg, dir, entry)
	return out, nil
}

// missingPorts names each port of the entry the package lost, at the use
// case's owned file, with the command that drops it.
func missingPorts(pkg areaPackage, dir string, entry manifest.Entry) []genkind.Decision {
	kept := slices.DeleteFunc(slices.Clone(entry.Ports), func(port string) bool { return !pkg.Types[port] })
	var out []genkind.Decision
	for _, port := range entry.Ports {
		if pkg.Types[port] {
			continue
		}
		out = append(out, genkind.Decision{
			Path: dir + "/" + snake(entry.Type()) + ".go", Line: 1,
			Text: fmt.Sprintf("usecase %s consumes port %s, which %s no longer declares: declare it again or run golaw new usecase %s --ports %s", entry.Name, port, dir, entry.Name, strings.Join(kept, ",")),
		})
	}
	return out
}
